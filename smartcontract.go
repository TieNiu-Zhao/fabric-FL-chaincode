package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	pb "github.com/hyperledger/fabric-protos-go/peer"
) 

type Ciphertext struct {
	ax []int64 // 实部
	bx []int64 // 虚部 
}

type Proposal struct {
	NoisyModel          []float64  `json:"noisymodel"`          // 加噪模型 
	EncryptedModel      Ciphertext `json:"encryptedmodel"`      // 加密模型
	EncryptedNoisy      Ciphertext `json:"encryptenoisy"`       // 加密噪声
	EncryptedNoisyModel Ciphertext `json:"encryptednoisymodel"` // 加密加噪模型
}

type UpdateRequest struct {
	EncryptedModel Ciphertext           `json:"encryptedmodel"` // 来自 proposal 的 EncryptedModel
	Endorsements   []*pb.Endorsement 	`json:"endorsements"`   // 来自 endorsing peers 的结果
}

type SmartContract struct {
}

var num int = 10	// update数，初始为10
var q int64 = 800	// 模数q

func (s *SmartContract) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

func (s *SmartContract) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	// 获取函数名和参数
	function, args := stub.GetFunctionAndParameters()

	
}

// ProposeUpdate 从客户端接收更新加密模型的提议，并将其广播到认可的 Peers 
func (s *SmartContract) ProposeUpdate(stub shim.ChaincodeStubInterface, proposal *Proposal) pb.Response { 
	// 检查提议是否为空 
	if proposal == nil {
		return shim.Error("提议不能为空")
	}
	// 对提议中的加噪模型进行投毒检测
	if !multikrum(proposal.NoisyModel) {
		num--
		return shim.Error("投毒检测不通过")
	}

	// 使用 addCipher 方法将提议中的加密模型与加密噪声相加
	sum := addCipher(proposal.EncryptedModel, proposal.EncryptedNoisy)

	// 检查加密模型与加密噪声的同态加法是否与提议中的加密加噪模型相等
	if !equal(sum, proposal.EncryptedNoisyModel) {
		num--
		return shim.Error("同态加法验证不通过")
	}

	// 如果验证通过，获取当前交易的背书结果
	endorsements, err := stub.GetEndorsements()
	if err != nil {
		return shim.Error(fmt.Sprintf("获取背书结果失败: %s", err))
	}

	// 将背书结果与提议中的加密模型拼装成更新请求
	update := &UpdateRequest{
		EncryptedModel: proposal.EncryptedModel,
		Endorsements:   endorsements,
	}

	// 将更新请求序列化为字节
	updateBytes, err := json.Marshal(update)
	if err != nil {
		return shim.Error(fmt.Sprintf("序列化更新请求失败: %s", err))
	}

	// 调用 upload 方法将更新请求发送给排序节点
	resp := s.upload(stub, updateBytes)
	if resp.Status != shim.OK {
		return resp
	}

	return shim.Success(nil)
}

// upload 方法将更新请求发送给排序节点，并等待排序结果
func (s *SmartContract) upload(stub shim.ChaincodeStubInterface, update []byte) pb.Response { 
	// 检查更新请求是否为空
	if update == nil {
		return shim.Error("更新请求不能为空")
	}

	// 创建一个新的交易提案，指定链码名称、通道名称、函数名和参数
	prop, _, err := stub.CreateProposalFromBytes("mycc", stub.GetChannelID(), "upload", [][]byte{update})
	if err != nil {
		return shim.Error(fmt.Sprintf("创建交易提案失败: %s", err))
	}

	// 将交易提案发送给排序节点，并等待排序结果
	resp, err := stub.SendProposalToOrderer(prop)
	if err != nil {
		return shim.Error(fmt.Sprintf("发送交易提案失败: %s", err))
	}

	// 检查排序结果是否成功
	if resp.Status != shim.OK {
		return resp
	}

	// 从排序结果中获取更新请求的数组
	var updates []*UpdateRequest
	err = json.Unmarshal(resp.Payload, &updates)
	if err != nil {
		return shim.Error(fmt.Sprintf("反序列化更新请求失败: %s", err))
	}

	// 检查更新请求的数量是否等于 num
	if len(updates) != num {
		return shim.Error(fmt.Sprintf("更新请求的数量不匹配: 预期 %d, 实际 %d", num, len(updates)))
	}

	// 使用 addCipher 方法对 num 个更新请求中的加密模型进行累加
	sum := Ciphertext{}
	for i := 0; i < num; i++ {
		sum = addCipher(sum, updates[i].EncryptedModel)
	}

	// 将累加结果上传到最新区块上
	err = stub.PutState("latest_model", sum)
	if err != nil {
		return shim.Error(fmt.Sprintf("failed to put state: %s", err))
	}

	return shim.Success(nil)
}

// query 方法根据键查询最新块的内容
func (s *SmartContract) query(stub shim.ChaincodeStubInterface, key string) pb.Response { 
	// 检查键是否为空 
	if key == “” { 
		return shim.Error("键不能为空") 
	}

	// 根据键从最新块中获取值
	value, err := stub.GetState(key)
	if err != nil {
		return shim.Error(fmt.Sprintf("获取状态失败: %s", err))
	}

	// 检查值是否为空
	if value == nil {
		return shim.Error("找不到对应的值")
	}

	// 将值反序列化为Ciphertext结构体
	var ciphertext Ciphertext
	err = json.Unmarshal(value, &ciphertext)
	if err != nil {
		return shim.Error(fmt.Sprintf("反序列化失败: %s", err))
	}

	// 将Ciphertext结构体的ax部分保存到一个私有数据集合中
	axBytes, err := json.Marshal(ciphertext.ax)
	if err != nil {
		return shim.Error(fmt.Sprintf("序列化失败: %s", err))
	}
	err = stub.PutPrivateData("axCollection", key, axBytes)
	if err != nil {
		return shim.Error(fmt.Sprintf("保存私有数据失败: %s", err))
	}

	// 返回Ciphertext结构体的bx部分
	return shim.Success([]byte(fmt.Sprintf("bx: %v", ciphertext.bx)))
}

// Decrypt 方法从客户端接收解密份额，并将其与私有数据中的ax部分相加，得到明文的解密结果，并上传到最新区块
func (s *SmartContract) Decrypt(stub shim.ChaincodeStubInterface, shares []*Ciphertext) pb.Response { 
	// 检查解密份额是否为空 
	if shares == nil { 
		return shim.Error("解密份额不能为空")
	}

	// 检查解密份额的数量是否等于 num
	if len(shares) != num {
		return shim.Error(fmt.Sprintf("解密份额的数量不匹配: 预期 %d, 实际 %d", num, len(shares)))
	}

	// 从私有数据集合中获取之前保存的ax部分
	axBytes, err := stub.GetPrivateData("axCollection", "latest_model")
	if err != nil {
		return shim.Error(fmt.Sprintf("获取私有数据失败: %s", err))
	}

	// 检查ax部分是否为空
	if axBytes == nil {
		return shim.Error("找不到对应的ax部分")
	}

	// 将ax部分反序列化为整数数组
	var ax []int64
	err = json.Unmarshal(axBytes, &ax)
	if err != nil {
		return shim.Error(fmt.Sprintf("反序列化失败: %s", err))
	}

	// 创建一个Ciphertext结构体，用来存储ax部分和空的bx部分
	cipher := &Ciphertext{
		ax: ax,
		bx: []int64{},
	}

	// 使用 addCipher 方法对 num 个解密份额进行累加
	sum := Ciphertext{}
	for i := 0; i < num; i++ {
		sum = addCipher(sum, shares[i])
	}

	// 将累加结果与ax部分相加，得到明文的解密结果
	result := addCipher(sum, cipher)

	// 将明文的解密结果序列化为字节
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return shim.Error(fmt.Sprintf("序列化失败: %s", err))
	}

	// 将明文的解密结果上传到最新区块上
	err = stub.PutState("latest_model", resultBytes)
	if err != nil {
		return shim.Error(fmt.Sprintf("上传状态失败: %s", err))
	}

	// 将 num 恢复为 10
	num = 10

	return shim.Success(nil)
}

func multikrum(noisyModel []float64) bool {
	var l2Norms []float64
	for _, param := range noisyModel {
		l2Norms = append(l2Norms, math.Pow(param, 2))
	}
	avgL2Norm := 0.0
	for _, l2Norm := range l2Norms {
		avgL2Norm += l2Norm
	}
	avgL2Norm /= float64(len(l2Norms))
	var diffSquares []float64
	for _, l2Norm := range l2Norms {
		diffSquares = append(diffSquares, math.Pow(l2Norm-avgL2Norm, 2))
	}
	avgDiffSquare := 0.0 
	for _, diffSquare := range diffSquares {
		avgDiffSquare += diffSquare
	}
	avgDiffSquare /= float64(len(diffSquares))
	stdDev := math.Sqrt(avgDiffSquare)
	avgStdDev := stdDev / float64(len(l2Norms))
	var ratios []float64
	for _, l2Norm := range l2Norms {
		ratios = append(ratios, stdDev/(l2Norm-avgL2Norm+avgStdDev))
	}
	avgRatio := 0.0
	for _, ratio := range ratios {
		avgRatio += ratio
	}
	avgRatio /= float64(len(ratios))
	// 判断是否投毒
	if avgRatio > 1 {
		return true
	}
	return false
}

func addCipher(cipherA, cipherB Ciphertext) Ciphertext {
	var cipherC Ciphertext 
	for i := 0; i < len(cipherA.ax); i++ {
		cipherC.ax[i] = (cipherA.ax[i] + cipherB.ax[i]) % q // 实部相加取模
		cipherC.bx[i] = (cipherA.bx[i] + cipherB.bx[i]) % q // 虚部相加取模 
	}
	return cipherC 
}

func equal(cipherA, cipherB Ciphertext) bool {
	for i := 0; i < len(cipherA.ax); i++ {
		if cipherA.ax[i] != cipherB.ax[i] || cipherA.bx[i] != cipherB.bx[i] {
			return false // 如果有一个系数不相等，返回false 
		} 
	}
	return true
}

func main() {
	err := shim.Start(new(SmartContract))
	if err != nil {
		fmt.Printf("Error starting SmartContract chaincode: %s", err)
	}
}

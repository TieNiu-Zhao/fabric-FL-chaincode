package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/pkg/errors"
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
	Endorsements   []*peer.Endorsement `json:"endorsements"`   	// 来自 endorsing peers 的结果
}

type SmartContract struct {
	contractapi.Contract
}

var num int = 10	// update数，初始为10
var q int64 = 800	// 模数q

// ProposeUpdate 从客户端接收更新加密模型的提议，并将其广播到认可的 Peers
func (s *SmartContract) ProposeUpdate(ctx contractapi.TransactionContextInterface, proposal *Proposal) (*Proposal, error) {
	// 验证来自客户端的建议
	err := s.ValidateProposal(ctx, proposal)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate proposal")
	}

	if !multikrum(proposal.NoisyModel) {
		num--
		return nil, errors.New("投毒检测不通过") 
	}
	
	// 使用 addCipher 方法将 proposal.EncryptedModel 与 proposal.EncryptedNoisy 相加
	sum := addCipher(proposal.EncryptedModel, proposal.EncryptedNoisy)

	if !equal(sum, EncryptedNoisyModel) {
		num--
		return nil, errors.New("同态加法验证不通过") 
	}

	// 将提案广播给赞同的 Peers
	// 获取链码背书策略
	ep, err := sdk.GetEndorsementPolicy("mychaincode")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get endorsement policy")
	}

	// 从背书策略中获取 Endorsing Peers
	endorsers, err := ep.GetEndorsers()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get endorsers")
	}

	// 用提案创建一个提案请求
	pr, err := sdk.NewProposalRequest(proposal)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create proposal request")
	}

	// 将提案请求发送到支持节点，并获得它们的响应
	responses, err := sdk.SendProposal(pr, endorsers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send proposal")
	}

	// 从响应中提取背书
	endorsements := make([]*peer.Endorsement, len(responses))
	for i, r := range responses {
		endorsements[i] = r.Endorsement
	}

	// 使用加密模型和背书创建更新请求
	update := &UpdateRequest{
		EncryptedModel: proposal.EncryptedModel,
		Endorsements:   endorsements,
	}

	// 将更新请求发送给排序节点
    err = s.SendUpdate(ctx, update)
    if err != nil {
        return nil, errors.Wrap(err, "failed to send update")
    }

    // 调用 OrderUpdate 方法
    response, err := ctx.GetStub().InvokeChaincode("mychaincode", [][]byte{[]byte("OrderUpdate"), []byte(update)}, "mychannel")
    if err != nil {
        return nil, errors.Wrap(err, "failed to invoke OrderUpdate")
    }

    // 检查响应是否有效
    if response.Status != peer.TxValidationCode_VALID {
        return nil, errors.New("OrderUpdate was not valid")
    }

	return response.Payload.([]byte), nil
}

func (s *SmartContract) OrderUpdate(ctx contractapi.TransactionContextInterface, updates []*UpdateRequest) error {
	// 验证来自客户端的更新请求
	err := s.ValidateUpdates(ctx, updates)
	if err != nil {
		return errors.Wrap(err, “failed to validate updates”)
	}
	// 使用 addCipher 方法对 num 个更新请求中的加密模型进行累加
	sum := Ciphertext{}
	for i := 0; i < num; i++ {
		sum = addCipher(sum, updates[i].EncryptedModel)
	}

	// 将累加结果上传到最新区块上
	err = s.PutState(ctx, "latest_model", sum)
	if err != nil {
		return errors.Wrap(err, "failed to put state")
	}

	// 将 num 恢复为 10
	num = 10
	return nil
}

// 定义一个查询函数，只返回 Ciphertext 里的 bx 部分 
func (s *SmartContract) QueryBx(ctx contractapi.TransactionContextInterface) (string, error) {
	// 从状态数据库中获取最新的加密模型 
	encryptedModel, err := ctx.GetStub().GetState(“latest_model”) 
	if err != nil { 
		return “”, errors.Wrap(err, “failed to get state”) 
	}

	// 将加密模型转换为 Ciphertext 结构体
	var ciphertext Ciphertext
	err = json.Unmarshal(encryptedModel, &ciphertext)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal ciphertext")
	}

	// 只返回 bx 部分
	return ciphertext.Bx, nil
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

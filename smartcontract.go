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
	NoisyModel           []float64 	`json:"noisymodel"`			// 加噪模型 
	EncryptedModel       Ciphertext	`json:"encryptedmodel"`		// 加密模型
	EncryptedNoisy       Ciphertext	`json:"encryptenoisy"`		// 加密噪声
	EncryptedNoisyModel  Ciphertext `json:"encryptednoisymodel"`// 加密加噪模型
}

type Endorsement struct {
	Endorser    string `json:"endorser"`
	Signature   []byte `json:"signature"`
}

type Update struct {
	Endorsements     []Endorsement 	`json:"endorsements"` 	// 背书结果
	EncryptedModel   ckkscipher     `json:"encryptedmodel"`	// 加密模型
}

var num int 			// 通过背书的客户端数量 
var q int64				// 同态加密的模参数 
var sum Ciphertext 		// 密文求和结果

type Chaincode struct {
}


func (c *Chaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

func (c *Chaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()

	if function == "endorsement" {
		if len(args) != 1 {
			return shim.Error("Incorrect number of arguments. Expecting 1")
		}

		var proposal Proposal
		err := json.Unmarshal([]byte(args[0]), &proposal)
		if err != nil {
			return shim.Error(err.Error())
		}

		signature, err := c.Endorsement(proposal)
		if err != nil {
			return shim.Error(err.Error())
		}

		update := Update{
			Endorsements:   []Endorsement{{Endorser: mspID, Signature: signature}},
			EncryptedModel: proposal.EncryptedModel,
		}

		updateBytes, err := json.Marshal(update)
		if err != nil {
			return shim.Error(err.Error())
		}

		return shim.Success(updateBytes)
	}

	return shim.Error("Invalid invoke function name. Expecting \"endorsement\"")
}

func (c *Chaincode) Endorsement(stub shim.ChaincodeStubInterface) pb.Response {
	proposal, err := getProposal(stub) 
	if err != nil { 
		return shim.Error(err.Error()) 
	}

	if !multikrum(proposal.NoisyModel) {
		return shim.Error("投毒检测不通过") 
	}

	if !equal(addCipher(proposal.EncryptedModel, proposal.Sum), sum) {
		return shim.Error("同态加法验证不通过") 
	}

	num++
	sum = addCipher(sum, proposal.EncryptedModel)
	signature, err := sign(proposal) 
	if err != nil {
		return shim.Error(err.Error()) 
	}

	return shim.Success(signature) 
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

func addCipher(cipherA, cipherB Ciphertext, q int64) Ciphertext {
	var cipherC Ciphertext 
	for i := 0; i < len(cipherA.ax); i++ {
		cipherC.ax[i] = (cipherA.ax[i] + cipherB.ax[i]) % q // 实部相加取模
		cipherC.bx[i] = (cipherA.bx[i] + cipherB.bx[i]) % q // 虚部相加取模 
	} 
	return cipherC 
}

// upload()函数，用于上传密文求和结果到最新区块 
func (c *Chaincode) upload(stub shim.ChaincodeStubInterface) pb.Response {
	update, err := getUpdate(stub)
	if err != nil {
		return shim.Error(err.Error()) 
	}
	if !verify(update.Endorsements) {
		return shim.Error("背书结果验证不通过") 
	} 
	// 同态加法求和，并上传到最新区块 
	var total Ciphertext // 总和密文 
	for i := 0; i < num; i++ {
		total = addCipher(total, update.EncryptedModel[i]) 
	}
	err = stub.PutState(“total”, total) 
	// 将总和密文存储到最新区块的状态数据库中 
	if err != nil { 
		return shim.Error(err.Error()) 
	}

	return shim.Success(nil)
}

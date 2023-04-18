package main

import (
	"encoding/json"
	"fmt"

	"github.com/ldsec/lattigo/ckks"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	pb "github.com/hyperledger/fabric-protos-go/peer"
)

type ckkscipher struct {
	values []complex128		// 密文的实部和虚部
    scale  float64			// 缩放因子
}

type Proposal struct {
	NoisyModel           []float64 	`json:"noisymodel"`			// 加噪模型 
	EncryptedModel       ckkscipher	`json:"encryptedmodel"`		// 加密模型
	EncryptedNoisy       ckkscipher	`json:"encryptenoisy"`		// 加密噪声
	EncryptedNoisyModel  ckkscipher `json:"encryptednoisymodel"`// 加密加噪模型
}

type Endorsement struct {
	Endorser    string `json:"endorser"`
	Signature   []byte `json:"signature"`
}

type Update struct {
	Endorsements     []Endorsement 	`json:"endorsements"` 	// 背书结果
	EncryptedModel   ckkscipher     `json:"encryptedmodel"`	// 加密模型
}

var num int = 10 			// 客户端数量

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

func (c *Chaincode) Endorsement(proposal Proposal) ([]byte, error) {
	mspID := "mspID" // 定义 mspID
	msp := "msp"     // 定义 msp
	
	if multikrum(proposal.NoisyModel) {
		log.Println("模型投毒！！！")
		return nil, nil
	}

	res := make([]byte, len(proposal.EncryptedModel))
	for i := range res {
		res[i] = proposal.EncryptedModel[i] + proposal.EncryptedNoisy[i]
	}
	if !bytes.Equal(res, proposal.EncryptedNoisyModel) {
		log.Println("不满足加性同态！！！")
		return nil, nil
	}

	endorsementResult := true
	endorsers := proposal.Endorsers
	for _, endorser := range endorsers {
		result := VerifyEndorsement(proposal, endorser)
		if !result {
			endorsementResult = false
			break
		}
	}
	if !endorsementResult {
		num--
	}
	endorsement := Endorsement{
		Endorser:  mspID,
		Signature: []byte{},
	}
	signer, err := msp.GetDefaultSigningIdentity()
	if err != nil {
		return nil, err
	}
	signature, err := signer.Sign(endorsement)
	if err != nil {
		return nil, err
	}
	return signature, nil
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

func addCipher(c1, c2 ckkscipher, context *ckkscontext) ckkscipher {
    if len(c1.values) != len(c2.values) {
        panic("ciphertexts must have the same length")
    }
    var result ckkscipher
    result.values = make([]complex128, len(c1.values))
    for i := range c1.values {
        result.values[i] = c1.values[i] + c2.values[i]
    }
    result.scale = c1.scale
    result.values = context.ModDown(result.values)
    return result
}

func (c *Chaincode) upload(stub shim.ChaincodeStubInterface, update Update) error {

}
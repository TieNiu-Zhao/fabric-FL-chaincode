package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/pkg/errors"
)

// Ciphertext represents a complex number with real and imaginary parts
type Ciphertext struct {
	ax []int64 // Real part
	bx []int64 // Imaginary part
}

// Proposal represents a proposal for updating the encrypted model
type Proposal struct {
	NoisyModel          []float64  `json:"noisymodel"`          // Noisy model
	EncryptedModel      Ciphertext `json:"encryptedmodel"`      // Encrypted model
	EncryptedNoisy      Ciphertext `json:"encryptenoisy"`       // Encrypted noise
	EncryptedNoisyModel Ciphertext `json:"encryptednoisymodel"` // Encrypted noisy model
}

// UpdateRequest represents a request for updating the encrypted model with endorsements
type UpdateRequest struct {
	EncryptedModel Ciphertext           `json:"encryptedmodel"` // The encrypted model from the proposal
	Endorsements   []*peer.Endorsement `json:"endorsements"`   // The endorsements from the endorsing peers
}

// SmartContract provides functions for managing an encrypted model
type SmartContract struct {
	contractapi.Contract
}

// ProposeUpdate从客户端接收更新加密模型的提议，并将其广播到认可的 Peers
func (s *SmartContract) ProposeUpdate(ctx contractapi.TransactionContextInterface, proposal *Proposal) (*Proposal, error) {
	// 验证来自客户端的建议
	err := s.ValidateProposal(ctx, proposal)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate proposal")
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

	// 从回答中提取背书
	endorsements := make([]*peer.Endorsement, len(responses))
	for i, r := range responses {
		endorsements[i] = r.Endorsement
	}

	// 使用加密模型和背书创建更新请求
	update := &UpdateRequest{
		EncryptedModel: proposal.EncryptedModel,
		Endorsements:   endorsements,
	}

	// 用更新请求创建一个更新请求事务
	urt, err := sdk.NewUpdateRequestTransaction(update)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create update request transaction")
	}

	// 将更新请求事务发送到排序服务并获得其响应
	response, err := sdk.SendTransaction(urt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send transaction")
	}

	// 检查响应状态
	if response.Status != peer.TxValidationCode_VALID {
		return nil, errors.New("transaction was not valid")
	}

	return response, nil
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

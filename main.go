package main

import ( 
	"encoding/json" 
	"fmt"
	"io/ioutil"
	"os"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
)

// Proposal is the structure of the proposal json file 
type Proposal struct { 
	EncryptedModel 		string json:"encryptedModel" 
	EncryptedNoisy 		string json:"encryptedNoisy" 
	EncryptedNoisyModel string json:"encryptedNoisyModel" 
	NoisyModel 			string json:"noisyModel" 
}

// UpdateRequest is the structure of the update request
type UpdateRequest struct { 
	EncryptedModel 	string 				json:"encryptedModel" 
	Endorsements 	[]*fab.Endorsement 	json:"endorsements" 
}

func main() {
	// 创建一个 fabric-sdk-go 的配置对象
	configProvider := config.FromFile(“./config.yaml”)

	// 创建一个 fabric-sdk-go 的实例
	sdk, err := fabsdk.New(configProvider)
	if err != nil {
		fmt.Printf("创建 sdk 失败: %s\n", err)
		os.Exit(1)
	}
	defer sdk.Close()

	// 创建一个 channel.Client 实例
	clientChannelContext := sdk.ChannelContext("mychannel", fabsdk.WithUser("User1"), fabsdk.WithOrg("Org1"))
	client, err := channel.New(clientChannelContext)
	if err != nil {
		fmt.Printf("创建 channel 客户端失败: %s\n", err)
		os.Exit(1)
	}

    // // Create a ledger client
    // ledgerClient, err := ledger.New(clientChannelContext)
    // if err != nil {
    //     fmt.Printf("Failed to create new ledger client: %s\n", err)
    //     os.Exit(1)
    // }

    // 循环读取本地文件 proposal1~10.json，并上传到区块链上
	for i := 1; i <= 10; i++ {
		fileName := fmt.Sprintf("proposal%d.json", i)
		fileBytes, err := ioutil.ReadFile(fileName)
		if err != nil {
			fmt.Printf("读取文件 %s 失败: %s\n", fileName, err)
			continue
		}

		var proposal *Proposal
		err = json.Unmarshal(fileBytes, &proposal)
		if err != nil {
			fmt.Printf("反序列化文件 %s 失败: %s\n", fileName, err)
			continue
		}

		transientMap := make(map[string][]byte)
		transientMap["proposal"] = fileBytes

		request := channel.Request{
			ChaincodeID: "mycc",
			Fcn:         "ProposeUpdate",
			Args:        [][]byte{},
			TransientMap: transientMap,
		}

		resp, err := client.SendRequest(request)
		if err != nil {
			fmt.Printf("发送交易提案失败: %s\n", err)
			continue
		}

		if resp.Status == fab.StatusOK {
			fmt.Printf("上传文件 %s 成功\n", fileName)
			fmt.Printf("响应内容: %s\n", string(resp.Payload))
		} else {
			fmt.Printf("上传文件 %s 失败\n", fileName)
			fmt.Printf("响应状态: %d\n", resp.Status)
			fmt.Printf("响应消息: %s\n", resp.Message)
			continue
		}
	}

	// 查询最新区块的 bx
	request := channel.Request{
		ChaincodeID: "mycc",
		Fcn:         "query",
		Args:        [][]byte{"latest_model"},
	}

	resp, err := client.Query(request)
	if err != nil {
		fmt.Printf("查询最新区块的 bx 失败: %s\n", err)
		os.Exit(1)
	}

	if resp.Status == fab.StatusOK {
		fmt.Printf("查询最新区块的 bx 成功\n")
		fmt.Printf("响应内容: %s\n", string(resp.Payload))
	} else {
		fmt.Printf("查询最新区块的 bx 失败\n")
		fmt.Printf("响应状态: %d\n", resp.Status)
		fmt.Printf("响应消息: %s\n", resp.Message)
		os.Exit(1)
	}
}

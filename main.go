package main

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
)

func main() {
	// Load the network configuration file
	configProvider := config.FromFile("config.yaml")

	// Create a Fabric SDK instance
	sdk, err := fabsdk.New(configProvider)
	if err != nil {
		log.Fatal("Failed to create new SDK: ", err)
	}
	defer sdk.Close()

	// Create a channel client for org1
	clientChannelContext := sdk.ChannelContext("mychannel", fabsdk.WithUser("User1"), fabsdk.WithOrg("Org1"))
	client, err := channel.New(clientChannelContext)
	if err != nil {
		log.Fatal("Failed to create new channel client: ", err)
	}

	// Loop through the proposal files
	for i := 1; i <= 10; i++ {
		// Read the proposal file
		fileName := fmt.Sprintf("proposal%d.json", i)
		fileContent, err := ioutil.ReadFile(fileName)
		if err != nil {
			log.Printf("Failed to read file %s: %v", fileName, err)
			continue
		}

		// Invoke the chaincode function with the proposal as argument
		response, err := client.Execute(channel.Request{
			ChaincodeID: "flcc",
			Fcn:         "update",
			Args:        [][]byte{fileContent},
		})
		if err != nil {
			log.Printf("Failed to invoke chaincode for file %s: %v", fileName, err)
			continue
		}

		// Print the response
		log.Printf("Response from file %s: %s", fileName, response.Payload)
	}
}
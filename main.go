package main

import ( 
	"encoding/json" 
	"fmt"
	"io/ioutil"
	"os"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
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
	// Load the configuration file 
	configProvider := config.FromFile(“./config.yaml”)

	// Create a Fabric SDK instance
	sdk, err := fabsdk.New(configProvider)
	if err != nil {
		fmt.Printf("Failed to create new SDK: %s\n", err)
		os.Exit(1)
	}
	defer sdk.Close()

	// Create a channel client
	clientChannelContext := sdk.ChannelContext("mychannel", fabsdk.WithUser("User1"), fabsdk.WithOrg("Org1"))
	client, err := channel.New(clientChannelContext)
	if err != nil {
		fmt.Printf("Failed to create new channel client: %s\n", err)
		os.Exit(1)
	}

    // Create a ledger client
    ledgerClient, err := ledger.New(clientChannelContext)
    if err != nil {
        fmt.Printf("Failed to create new ledger client: %s\n", err)
        os.Exit(1)
    }

    // Loop through the proposal files
	for i := 1; i <= 10; i++ {
		// Read the proposal file
		filename := fmt.Sprintf("proposal%d.json", i)
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			fmt.Printf("Failed to read file %s: %s\n", filename, err)
			os.Exit(1)
		}

		// Unmarshal the proposal data
		var proposal Proposal
		err = json.Unmarshal(data, &proposal)
		if err != nil {
			fmt.Printf("Failed to unmarshal proposal data: %s\n", err)
			os.Exit(1)
		}

		// Prepare the arguments for invoking the chaincode
		args := [][]byte{[]byte(proposal.EncryptedModel), []byte(proposal.EncryptedNoisy), []byte(proposal.EncryptedNoisyModel), []byte(proposal.NoisyModel)}

		// Invoke the chaincode with ProposeUpdate function
		response, err := client.Execute(channel.Request{ChaincodeID: "mychaincode", Fcn: "ProposeUpdate", Args: args})
		if err != nil {
			fmt.Printf("Failed to invoke chaincode: %s\n", err)
			os.Exit(1)
		}

		// Check the response status
		if response.ChaincodeStatus != 200 {
			fmt.Printf("Chaincode invocation failed: %s\n", response.Info)
			os.Exit(1)
		}

		// Print the response payload
		fmt.Printf("Response from chaincode: %s\n", string(response.Payload))
    }

    // Query the latest block from the ledger
    block, err := ledgerClient.QueryBlockByTxID(fab.TransactionID(response.TransactionID))
    if err != nil {
        fmt.Printf("Failed to query block by txid: %s\n", err)
        os.Exit(1)
    }

    // Print the block number and data hash
    fmt.Printf("Latest block number: %d\n", block.Header.Number)
    fmt.Printf("Latest block data hash: %x\n", block.Header.DataHash)
}

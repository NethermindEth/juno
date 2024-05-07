package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

/*
- getTxnReceipt Done.
*/
func main() {
	juno_getBlockWithTxsAndReceipts()
}

// juno_getBlockWithTxsAndReceipts
func juno_getBlockWithTxsAndReceipts() {
	exportRPCResponse("starknet_getBlockWithReceipts", "/cmd/juno/rpc_respReceipts.json")
	exportRPCResponse("starknet_getBlockWithTxs", "/cmd/juno/rpc_respTxs.json")
}

func exportRPCResponse(method, path string) {
	// 1. Making Payload
	reqData := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params": map[string]interface{}{
			"block_id": "latest", // took latest block
		},
		"id": 1,
	}

	// Marshalling request body
	reqBody, err := json.Marshal(reqData)
	if err != nil {
		fmt.Printf("Error marshalling JSON (%s): %v\n", method, err)
		return
	}

	// Making POST request to port: 6060
	resp, err := http.Post("http://localhost:6060", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		fmt.Println("Error making POST request:", err)
		return
	}
	defer resp.Body.Close()

	// Get the directory of the pps_rpc.go fileReceipt
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current directory:", err)
		return
	}
	filePathReceipt := filepath.Join(dir, path) // location same as code.

	// Create a new fileReceipt to write the JSON data
	file, err := os.Create(filePathReceipt)
	if err != nil {
		fmt.Println("Error creating file (Receipt):", err)
		return
	}
	defer file.Close()

	// Marshal the resReceipt + resTxs map into JSON and write it to the fileReceipt
	jsonData, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	_, err = file.Write(jsonData)
	if err != nil {
		fmt.Println("Error writing JSON to file (Receipt):", err)
		return
	}
	fmt.Println("RPC <<", method, ">> successfully exported to", filePathReceipt)
}

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
	// 1. Make payload
	requestData := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params": map[string]interface{}{
			"block_id": "latest",
		},
		"id": 1,
	}

	// 2. Marshalling back to JSON
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	// 3. Making POST request
	response, err := http.Post("http://localhost:6060", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Println("Error making POST request:", err)
		return
	}
	defer response.Body.Close()

	// 4. Read the response body
	var result map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}

	// Making proper files
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current directory:", err)
		return
	}
	filePath := filepath.Join(dir, path)
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	// Put JSONify data into it
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	_, err = file.Write(jsonData)
	if err != nil {
		fmt.Println("Error writing JSON to file:", err)
		return
	}

	fmt.Println("RPC response (", method, ") successfully exported to", filePath)
}

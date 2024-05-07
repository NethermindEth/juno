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
	// Define the RPC request payload
	requestData := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "starknet_getBlockWithReceipts",
		"params": map[string]interface{}{
			"block_id": "latest", // Replace with actual block identifier
		},
		"id": 1,
	}

	// Convert request data to JSON
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	// Make POST request to the RPC server
	response, err := http.Post("http://localhost:6060", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Println("Error making POST request:", err)
		return
	}
	defer response.Body.Close()

	// Read the response body
	var result map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}

	// Get the directory of the pps_rpc.go file
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current directory:", err)
		return
	}
	fmt.Println("Current directory:", dir)
	// Create the file path for the JSON file in the same directory
	filePath := filepath.Join(dir, "/cmd/juno/rpc_response.json")

	// Create a new file to write the JSON data
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	// Marshal the result map into JSON and write it to the file
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

	fmt.Println("RPC response successfully exported to", filePath)
}

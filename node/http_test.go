package node

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/NethermindEth/juno/rpc"
	"github.com/stretchr/testify/assert" // Use a testing library like testify/assert for assertions.
)

func TestMethods(t *testing.T) {
	// Create a mock rpc.Handler for testing purposes.
	mockHandler := &rpc.Handler{}

	// Call the methods function to get the list of JSON-RPC methods.
	rpcMethods := methods(mockHandler)

	// list of all expected rpc methods
	expectedMethods := []string{
		"starknet_chainId",
		"starknet_blockNumber",
		"starknet_blockHashAndNumber",
		"starknet_getBlockWithTxHashes",
		"starknet_getBlockWithTxs",
		"starknet_getTransactionByHash",
		"starknet_getTransactionReceipt",
		"starknet_getBlockTransactionCount",
		"starknet_getTransactionByBlockIdAndIndex",
		"starknet_getStateUpdate",
		"starknet_syncing",
		"starknet_getNonce",
		"starknet_getStorageAt",
		"starknet_getClassHashAt",
		"starknet_getClass",
		"starknet_getClassAt",
		"starknet_syncing",
		"starknet_getEvents",
		"starknet_call",
		"starknet_estimateFee",
		"starknet_addInvokeTransaction",
		"starknet_addDeclareTransaction",
		"starknet_addDeployAccountTransaction",
		"starknet_pendingTransactions",
		"starknet_traceTransaction",
		"starknet_traceBlockTransactions",
		"starknet_simulateTransactions",
	}
	// check all rpc methods are present
	for _, expectedMethod := range expectedMethods {
		found := false
		for _, method := range rpcMethods {
			if method.Name == expectedMethod {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected method '%s' not found in methods.", expectedMethod)
	}
	// Now you can write assertions to verify the correctness of the generated methods.

	// You can add more test cases for other methods as needed.

	// Example: Ensure the total number of methods is as expected.
	expectedMethodCount := 29 // Adjust the expected count based on your actual methods.
	assert.Equal(t, expectedMethodCount, len(rpcMethods), "Unexpected number of methods.")
}

// test function makeHTTPService
func TestMakeHTTPService(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	httpService := makeHTTPService(6060, handler)
	assert.NotNil(t, httpService)
}

// test makePPROF function

func TestHttpService_Run(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	httpService := makeHTTPService(6060, handler)
	assert.NotNil(t, httpService)
	closed := false
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	go func() {
		err := httpService.Run(ctx)
		assert.NoError(t, err)
		closed = true
	}()
	time.Sleep(5 * time.Second)
	assert.True(t, closed)
}

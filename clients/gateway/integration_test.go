//go:build integration
// +build integration

package gateway_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var network = utils.MAINNET

func init() {
	flag.Var(&network, "network", "network to use")
}

func TestClientIntegration(t *testing.T) {
	log, err := utils.NewZapLogger(utils.DEBUG, true)
	require.NoError(t, err)

	client := gateway.NewClient(network.GatewayURL(), log)

	handler := rpc.New(nil, nil, utils.MAINNET, client, nil)

	t.Run("Correct request", func(t *testing.T) {
		const gatewayRequest = `
		{
		  "type": "INVOKE_FUNCTION",
		  "version": "0x1",
		  "max_fee": "0x630a0aff77",
		  "signature": [
			"3528007825596418374026627280134684856450592690572806863856267180750827721823",
			"1245823039743824185280620435679602775503159522192240958005687298327230184621"
		  ],
		  "nonce": "0x2",
		  "sender_address": "0x3fdcbeb68e607c8febf01d7ef274cbf68091a0bd1556c0b8f8e80d732f7850f",
		  "calldata": [
			"1",
			"834014391734518171968827433472208778143213814961523717423700643029090972826",
			"1530486729947006463063166157847785599120665941190480211966374137237989315360",
			"0",
			"1",
			"1",
			"1"
		  ]
		}
		`
		var txn gateway.BroadcastedInvokeTxn
		err = json.Unmarshal([]byte(gatewayRequest), &txn)
		require.NoError(t, err)

		hash, err := client.AddInvokeTransaction(context.Background(), &txn)
		if assert.NoError(t, err) {
			fmt.Printf("AddInvokeTransaction result: %s\n", hash)
		}
	})
	t.Run("Incorrect request", func(t *testing.T) {
		const gatewayRequest = `
		{
		  "type": "INVOKE",
		  "version": "0x1",
		  "max_fee": "0x630a0aff77",
		  "signature": [
			"3528007825596418374026627280134684856450592690572806863856267180750827721823",
			"1245823039743824185280620435679602775503159522192240958005687298327230184621"
		  ],
		  "nonce": "0x2",
		  "sender_address": "0x3fdcbeb68e607c8febf01d7ef274cbf68091a0bd1556c0b8f8e80d732f7850f",
		  "calldata": [
			"1",
			"834014391734518171968827433472208778143213814961523717423700643029090972826",
			"1530486729947006463063166157847785599120665941190480211966374137237989315360",
			"0",
			"1",
			"1",
			"1"
		  ]
		}
		`
		var txn gateway.BroadcastedInvokeTxn
		err = json.Unmarshal([]byte(gatewayRequest), &txn)
		require.NoError(t, err)

		_, err := client.AddInvokeTransaction(context.Background(), &txn)
		require.Error(t, err)
		assert.NotEmpty(t, err.Error())
	})

	t.Run("Validation", func(t *testing.T) {
		const gatewayRequest = `{}`
		var txn gateway.BroadcastedInvokeTxn
		err = json.Unmarshal([]byte(gatewayRequest), &txn)
		require.NoError(t, err)

		_, err := handler.AddInvokeTransaction(&txn)
		assert.Equal(t, err.Code, jsonrpc.InvalidParams)
		assert.Equal(t, err.Message, "{'MaxFee': ['Missing data for required field.']}")
	})

	t.Run("Validation - incorrect version", func(t *testing.T) {
		const gatewayRequest = `
		{
		  "type": "INVOKE",
		  "version": "0x0",
		  "max_fee": "0x630a0aff77",
		  "signature": [
			"3528007825596418374026627280134684856450592690572806863856267180750827721823",
			"1245823039743824185280620435679602775503159522192240958005687298327230184621"
		  ],
		  "nonce": "0x2",
		  "sender_address": "0x3fdcbeb68e607c8febf01d7ef274cbf68091a0bd1556c0b8f8e80d732f7850f",
		  "calldata": [
			"1",
			"834014391734518171968827433472208778143213814961523717423700643029090972826",
			"1530486729947006463063166157847785599120665941190480211966374137237989315360",
			"0",
			"1",
			"1",
			"1"
		  ]
		}
		`
		var txn gateway.BroadcastedInvokeTxn
		err = json.Unmarshal([]byte(gatewayRequest), &txn)
		require.NoError(t, err)

		_, err := handler.AddInvokeTransaction(&txn)
		assert.Equal(t, err.Code, jsonrpc.InvalidParams)
		assert.Equal(t, err.Message, "Transaction version '0x0' not supported. Supported versions: '0x1'")
	})

	t.Run("Address out of range", func(t *testing.T) {
		const gatewayRequest = `
		{
		  "type": "INVOKE",
		  "version": "0x1",
		  "max_fee": "0x630a0aff77",
		  "signature": [
			"3528007825596418374026627280134684856450592690572806863856267180750827721823",
			"1245823039743824185280620435679602775503159522192240958005687298327230184621"
		  ],
		  "nonce": "0x2",
		  "sender_address": "0x0",
		  "calldata": [
			"1",
			"834014391734518171968827433472208778143213814961523717423700643029090972826",
			"1530486729947006463063166157847785599120665941190480211966374137237989315360",
			"0",
			"1",
			"1",
			"1"
		  ]
		}
		`
		var txn gateway.BroadcastedInvokeTxn
		err = json.Unmarshal([]byte(gatewayRequest), &txn)
		require.NoError(t, err)

		_, err := handler.AddInvokeTransaction(&txn)

		assert.Equal(t, err.Code, jsonrpc.InvalidParams)
		assert.Equal(t, err.Message, "contract address 0x0 is out of range")
	})

	t.Run("Fee out of range", func(t *testing.T) {
		const gatewayRequest = `
		{
		  "type": "INVOKE_FUNCTION",
		  "version": "0x1",
		  "max_fee": "0x3fdcbeb68e607c8febf01d7ef274cbf68091a0bd1556c0b8f8e80d732f7850f",
		  "signature": [
			"3528007825596418374026627280134684856450592690572806863856267180750827721823",
			"1245823039743824185280620435679602775503159522192240958005687298327230184621"
		  ],
		  "nonce": "0x2",
		  "sender_address": "0x3fdcbeb68e607c8febf01d7ef274cbf68091a0bd1556c0b8f8e80d732f7850f",
		  "calldata": [
			"1",
			"834014391734518171968827433472208778143213814961523717423700643029090972826",
			"1530486729947006463063166157847785599120665941190480211966374137237989315360",
			"0",
			"1",
			"1",
			"1"
		  ]
		}
		`
		var txn gateway.BroadcastedInvokeTxn
		err = json.Unmarshal([]byte(gatewayRequest), &txn)
		require.NoError(t, err)

		_, err := handler.AddInvokeTransaction(&txn)

		assert.Equal(t, err.Code, jsonrpc.InvalidParams)
		assert.Equal(t, err.Message, "Fee 0x3fdcbeb68e607c8febf01d7ef274cbf68091a0bd1556c0b8f8e80d732f7850f is out of range")
	})

	t.Run("Min Fee", func(t *testing.T) {
		const gatewayRequest = `
		{
		  "type": "INVOKE_FUNCTION",
		  "version": "0x1",
		  "max_fee": "0x0",
		  "signature": [
			"3528007825596418374026627280134684856450592690572806863856267180750827721823",
			"1245823039743824185280620435679602775503159522192240958005687298327230184621"
		  ],
		  "nonce": "0x2",
		  "sender_address": "0x3fdcbeb68e607c8febf01d7ef274cbf68091a0bd1556c0b8f8e80d732f7850f",
		  "calldata": [
			"1",
			"834014391734518171968827433472208778143213814961523717423700643029090972826",
			"1530486729947006463063166157847785599120665941190480211966374137237989315360",
			"0",
			"1",
			"1",
			"1"
		  ]
		}
		`
		var txn gateway.BroadcastedInvokeTxn
		err = json.Unmarshal([]byte(gatewayRequest), &txn)
		require.NoError(t, err)

		_, err := handler.AddInvokeTransaction(&txn)

		assert.Equal(t, err.Code, jsonrpc.InvalidParams)
		assert.Equal(t, err.Message, "max_fee must be bigger than 0.\n0 >= 0")
	})
}

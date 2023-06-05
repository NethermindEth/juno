package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateAddInvokeTx() *gateway.BroadcastedInvokeTxn {
	maxFee := new(felt.Felt).SetUint64(0x1)
	nonce := new(felt.Felt).SetUint64(1)
	senderAddress, _ := new(felt.Felt).SetString("0x326e3db4580b94948ca9d1d87fa359f2fa047a31a34757734a86aa4231fb9bb")

	return &gateway.BroadcastedInvokeTxn{
		BroadcastedTxnCmn: gateway.BroadcastedTxnCmn{
			MaxFee:    maxFee,
			Version:   "0x1",
			Signature: []*felt.Felt{},
			Nonce:     nonce,
		},
		Type:          "INVOKE",
		SenderAddress: senderAddress,
		Calldata:      []*felt.Felt{},
	}
}

func TestAddInvokeTx(t *testing.T) {
	client, closeFn := gateway.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)

	t.Run("Correct request", func(t *testing.T) {
		invokeTx := generateAddInvokeTx()
		invokeTxByte, err := json.Marshal(invokeTx)
		require.NoError(t, err)
		invokeTxRM := json.RawMessage(invokeTxByte)
		// Fix sender address so we know the transaction hash ahead of time for the test checks
		invokeTx.SenderAddress, _ = new(felt.Felt).SetString("0x326e3db4580b94948ca9d1d87fa359f2fa047a31a34757734a86aa4231fb9bb")
		expectedResp, _ := new(felt.Felt).SetString("0x5b113797c13a982b2bda3c52ed7fe31e494810c8937b3ec7ec4e0b21488ce87")

		resp, err := client.AddInvokeTransaction(context.Background(), &invokeTxRM)

		require.NoError(t, err)
		assert.Equal(t, expectedResp, resp)
	})

	t.Run("Incorrect empty request", func(t *testing.T) {
		invokeTx := &gateway.BroadcastedInvokeTxn{}
		invokeTxByte, err := json.Marshal(invokeTx)
		require.NoError(t, err)
		invokeTxRM := json.RawMessage(invokeTxByte)
		resp, err := client.AddInvokeTransaction(context.Background(), &invokeTxRM)

		assert.Nil(t, resp)
		assert.Containsf(t, err.Error(), "['Missing data for required field.']", "error message %s", "formatted")
	})

	t.Run("Incorrect version", func(t *testing.T) {
		invokeTx := generateAddInvokeTx()
		invokeTx.Version = "0x0"
		invokeTxByte, err := json.Marshal(invokeTx)
		require.NoError(t, err)
		invokeTxRM := json.RawMessage(invokeTxByte)
		resp, err := client.AddInvokeTransaction(context.Background(), &invokeTxRM)

		assert.Nil(t, resp)
		assert.Equal(t, err, errors.New("Transaction version 0x0 is not supported. Supported versions: [1]."))
	})

	t.Run("Incorrect max-fee", func(t *testing.T) {
		invokeTx := generateAddInvokeTx()
		invokeTx.MaxFee, _ = new(felt.Felt).SetString("0x0")
		invokeTxByte, err := json.Marshal(invokeTx)
		require.NoError(t, err)
		invokeTxRM := json.RawMessage(invokeTxByte)
		resp, err := client.AddInvokeTransaction(context.Background(), &invokeTxRM)

		assert.Nil(t, resp)
		assert.Equal(t, err, errors.New("max_fee must be bigger than 0.\n0 >= 0"))
	})
}

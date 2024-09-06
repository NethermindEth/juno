package gateway_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddInvokeTx(t *testing.T) {
	client := gateway.NewTestClient(t)

	t.Run("Correct request", func(t *testing.T) {
		invokeTx := `{"max_fee":"0x1","version":"0x1","signature":[],"nonce":"0x1","type":"INVOKE","sender_address":"0x326e3db4580b94948ca9d1d87fa359f2fa047a31a34757734a86aa4231fb9bb","calldata":[]}`

		invokeTxByte, err := json.Marshal(invokeTx)
		require.NoError(t, err)

		_, err = client.AddTransaction(context.Background(), invokeTxByte)

		// Since this method is just a proxy for the gateway we don't care what the actual response is,
		// we just need to check that no error is returned for a well-formed request.
		assert.NoError(t, err)
	})

	t.Run("Incorrect empty request", func(t *testing.T) {
		invokeTx := "{}"
		invokeTxByte, err := json.Marshal(invokeTx)
		require.NoError(t, err)
		resp, err := client.AddTransaction(context.Background(), invokeTxByte)

		require.Error(t, err)
		assert.Nil(t, resp)

		gatewayErr, ok := err.(*gateway.Error)
		require.True(t, ok)
		assert.Equal(t, gateway.ErrorCode("Malformed Request"), gatewayErr.Code)
		assert.Equal(t, "empty request", gatewayErr.Message)
	})

	t.Run("empty req", func(t *testing.T) {
		resp, err := client.AddTransaction(context.Background(), nil)

		require.EqualError(t, err, "500 Internal Server Error")
		assert.Nil(t, resp)
	})
}

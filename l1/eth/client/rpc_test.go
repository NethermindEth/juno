package client_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/stretchr/testify/assert"
)

func TestRPCError_Error(t *testing.T) {
	cases := []struct {
		name string
		err  client.RPCError
		want string
	}{
		{
			name: "without data",
			err:  client.RPCError{Code: -32601, Message: "the method does not exist"},
			want: "json-rpc error -32601: the method does not exist",
		},
		{
			name: "with data",
			err: client.RPCError{
				Code:    -32000,
				Message: "execution reverted",
				Data:    json.RawMessage(`"0xabcd"`),
			},
			want: `json-rpc error -32000: execution reverted: "0xabcd"`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.err.Error())
		})
	}
}

package client_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterQuery_MarshalShapes(t *testing.T) {
	addr := eth.AddressFromString("0x000000000000000000000000000000000000beef")
	hash1 := eth.HashFromString("0x" + repeatHex("11", 32))
	hash2 := eth.HashFromString("0x" + repeatHex("22", 32))

	cases := []struct {
		name   string
		q      client.FilterQuery
		assert func(t *testing.T, sent map[string]any)
	}{
		{
			// Unset FromBlock/ToBlock must not hit the wire: geth reads an
			// explicit toBlock=0 as a bounded filter ending at block 0.
			name: "unset block range omits keys",
			q:    client.FilterQuery{},
			assert: func(t *testing.T, sent map[string]any) {
				_, hasFrom := sent["fromBlock"]
				assert.False(t, hasFrom, "fromBlock must be omitted when unset")
				_, hasTo := sent["toBlock"]
				assert.False(t, hasTo, "toBlock must be omitted when unset")
				_, hasAddr := sent["address"]
				assert.False(t, hasAddr)
				_, hasTopics := sent["topics"]
				assert.False(t, hasTopics)
			},
		},
		{
			// Explicit zero is distinct from unset and still expressible
			// (e.g. eth_getLogs from genesis).
			name: "explicit block zero",
			q:    client.FilterQuery{FromBlock: ptr(uint64(0)), ToBlock: ptr(uint64(0))},
			assert: func(t *testing.T, sent map[string]any) {
				assert.Equal(t, "0x0", sent["fromBlock"])
				assert.Equal(t, "0x0", sent["toBlock"])
			},
		},
		{
			name: "single topic",
			q:    client.FilterQuery{Topics: [][]eth.Hash{{hash1}}},
			assert: func(t *testing.T, sent map[string]any) {
				topics := sent["topics"].([]any)
				require.Len(t, topics, 1)
				_, isString := topics[0].(string)
				assert.True(t, isString)
			},
		},
		{
			name: "any-at-position-0 then exact-at-1",
			q:    client.FilterQuery{Topics: [][]eth.Hash{nil, {hash1}}},
			assert: func(t *testing.T, sent map[string]any) {
				topics := sent["topics"].([]any)
				require.Len(t, topics, 2)
				assert.Nil(t, topics[0])
				_, isString := topics[1].(string)
				assert.True(t, isString)
			},
		},
		{
			name: "OR-list at position 0",
			q:    client.FilterQuery{Topics: [][]eth.Hash{{hash1, hash2}}},
			assert: func(t *testing.T, sent map[string]any) {
				topics := sent["topics"].([]any)
				_, isArr := topics[0].([]any)
				assert.True(t, isArr)
			},
		},
		{
			name: "addresses",
			q:    client.FilterQuery{Addresses: []eth.Address{addr}},
			assert: func(t *testing.T, sent map[string]any) {
				addrs := sent["address"].([]any)
				require.Len(t, addrs, 1)
				assert.Equal(t, addrHex(addr), addrs[0])
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := json.Marshal(c.q)
			require.NoError(t, err)
			var sent map[string]any
			require.NoError(t, json.Unmarshal(raw, &sent))
			c.assert(t, sent)
		})
	}
}

func repeatHex(unit string, repeat int) string {
	out := make([]byte, 0, len(unit)*repeat)
	for range repeat {
		out = append(out, unit...)
	}
	return string(out)
}

func addrHex(a eth.Address) string {
	b, _ := a.MarshalText()
	return string(b)
}

func ptr[T any](v T) *T { return &v }

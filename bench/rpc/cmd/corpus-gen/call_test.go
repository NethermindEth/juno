package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMulticallCalls(t *testing.T) {
	tests := []struct {
		name     string
		calldata []string
		want     []functionCall
	}{
		{
			name:     "two calls",
			calldata: []string{"0x2", "0xa", "0xb", "0x2", "0x1", "0x2", "0xc", "0xd", "0x0"},
			want: []functionCall{
				{
					ContractAddress:    "0xa",
					EntryPointSelector: "0xb",
					Calldata:           []string{"0x1", "0x2"},
				},
				{
					ContractAddress:    "0xc",
					EntryPointSelector: "0xd",
					Calldata:           []string{},
				},
			},
		},
		{
			name:     "no calls",
			calldata: []string{"0x0"},
			want:     []functionCall{},
		},
		{
			name:     "empty calldata",
			calldata: nil,
			want:     nil,
		},
		{
			name:     "count exceeds calldata",
			calldata: []string{"0x9", "0xa", "0xb", "0x0"},
			want:     nil,
		},
		{
			name:     "args exceed calldata",
			calldata: []string{"0x1", "0xa", "0xb", "0x4", "0x1"},
			want:     nil,
		},
		{
			name:     "trailing junk",
			calldata: []string{"0x1", "0xa", "0xb", "0x0", "0xff"},
			want:     nil,
		},
		{
			name:     "count wider than uint64",
			calldata: []string{"0x10000000000000000", "0xa", "0xb", "0x0"},
			want:     nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, multicallCalls(test.calldata))
		})
	}
}

func TestInvokeCalls(t *testing.T) {
	txs := []broadcastedTx{
		txFrom(t, `{"type":"INVOKE","version":"0x3","calldata":["0x1","0xa","0xb","0x0"]}`),
		txFrom(t, `{"type":"DEPLOY_ACCOUNT","version":"0x3"}`),
		txFrom(t, `{"type":"INVOKE","version":"0x3","calldata":["0xff"]}`),
		txFrom(t, `{"type":"INVOKE","version":"0x3","calldata":["0x1","0xc","0xd","0x1","0x9"]}`),
	}

	want := []functionCall{
		{ContractAddress: "0xa", EntryPointSelector: "0xb", Calldata: []string{}},
		{ContractAddress: "0xc", EntryPointSelector: "0xd", Calldata: []string{"0x9"}},
	}
	require.Equal(t, want, invokeCalls(txs))
}

func TestCallCorpus(t *testing.T) {
	node := newFakeNode(fakeInvokeTx, fakeL1HandlerTx)
	server := httptest.NewServer(node)
	defer server.Close()

	corpus := runCorpusGen(t, "call", "--count", "1", "--source-url", server.URL)

	require.Len(t, corpus.Requests, 1)
	entry := corpus.Requests[0]
	require.Equal(t, "starknet_call", entry.Method)

	var params struct {
		Request functionCall `json:"request"`
	}
	require.NoError(t, json.Unmarshal(entry.Params, &params))

	want := functionCall{
		ContractAddress:    "0xa",
		EntryPointSelector: "0xb",
		Calldata:           []string{},
	}
	require.Equal(t, want, params.Request)
	require.Equal(t, 1, node.verifications())
}

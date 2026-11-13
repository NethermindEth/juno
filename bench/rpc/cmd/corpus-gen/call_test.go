package main_test

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func invokeWithCalldata(calldata ...string) string {
	return `{"type":"INVOKE","version":"0x3","calldata":["` + strings.Join(calldata, `","`) + `"]}`
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

func TestCallSamplesEachMulticallCall(t *testing.T) {
	tests := []struct {
		name string
		txs  []string
		want []functionCall
	}{
		{
			name: "two calls in one transaction",
			txs: []string{
				invokeWithCalldata("0x2", "0xa", "0xb", "0x2", "0x1", "0x2", "0xc", "0xd", "0x0"),
			},
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
			name: "skips transactions without a multicall",
			txs: []string{
				invokeWithCalldata("0x1", "0xa", "0xb", "0x0"),
				`{"type":"DEPLOY_ACCOUNT","version":"0x3"}`,
				invokeWithCalldata("0xff"),
				invokeWithCalldata("0x1", "0xc", "0xd", "0x1", "0x9"),
			},
			want: []functionCall{
				{ContractAddress: "0xa", EntryPointSelector: "0xb", Calldata: []string{}},
				{ContractAddress: "0xc", EntryPointSelector: "0xd", Calldata: []string{"0x9"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(newFakeNode(test.txs...))
			defer server.Close()

			corpus := runCorpusGen(t, "call", "--count", "20", "--source-url", server.URL)

			var got []functionCall
			for _, entry := range corpus.Requests {
				var params struct {
					Request functionCall `json:"request"`
				}
				require.NoError(t, json.Unmarshal(entry.Params, &params))
				seen := slices.ContainsFunc(got, func(call functionCall) bool {
					return reflect.DeepEqual(call, params.Request)
				})
				if !seen {
					got = append(got, params.Request)
				}
			}
			require.ElementsMatch(t, test.want, got)
		})
	}
}

func TestCallRejectsCalldataWithoutMulticall(t *testing.T) {
	tests := map[string]string{
		"no calls":                 invokeWithCalldata("0x0"),
		"empty calldata":           `{"type":"INVOKE","version":"0x3","calldata":[]}`,
		"count exceeds calldata":   invokeWithCalldata("0x9", "0xa", "0xb", "0x0"),
		"args exceed calldata":     invokeWithCalldata("0x1", "0xa", "0xb", "0x4", "0x1"),
		"trailing junk":            invokeWithCalldata("0x1", "0xa", "0xb", "0x0", "0xff"),
		"count wider than uint64":  invokeWithCalldata("0x10000000000000000", "0xa", "0xb", "0x0"),
		"length wider than an int": invokeWithCalldata("0x1", "0xa", "0xb", "0x80000000"),
	}

	for name, tx := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(newFakeNode(tx))
			defer server.Close()

			err := corpusGenError(t, "call", "--count", "1", "--source-url", server.URL)
			require.Contains(t, err, "no account multicall in block")
		})
	}
}

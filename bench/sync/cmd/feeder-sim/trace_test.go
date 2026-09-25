package main

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/require"
)

func TestTraceNewRequest(t *testing.T) {
	rpcURL, err := url.Parse("http://node.example:6060/rpc/v0_10")
	require.NoError(t, err)

	tests := []struct {
		name    string
		query   url.Values
		wantErr string
	}{
		{"block number", mustResource(t, preConfirmedBlock, blockKey{BlockNumber: 56377}).query, ""},
		{"latest", url.Values{"blockNumber": {"latest"}}, "get_preconfirmed_block: "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trace := &traceAPI{url: rpcURL}
			request, err := trace.newRequest(t.Context(), resource{query: test.query})
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, http.MethodPost, request.Method)
			require.Equal(t, rpcURL.String(), request.URL.String())
			require.Equal(t, "application/json", request.Header.Get("Content-Type"))
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			require.JSONEq(t, `{
				"jsonrpc": "2.0", "id": 1, "method": "starknet_traceBlockTransactions",
				"params": {"block_id": {"block_number": 56377}}
			}`, string(body))
		})
	}
}

func TestDecodeTraces(t *testing.T) {
	result, err := os.ReadFile(traceFixture)
	require.NoError(t, err)
	want := decodeTraceResult(t, result)
	require.Len(t, want, 2)

	tests := []struct {
		name     string
		body     []byte
		encoding string
		want     []tracedTransaction
		wantErr  string
	}{
		{"identity", rpcResult(result), "identity", want, ""},
		{"unset encoding", rpcResult(result), "", want, ""},
		{"gzip", gzipped(t, rpcResult(result)), "gzip", want, ""},
		{"empty result", rpcResult([]byte("[]")), "", []tracedTransaction{}, ""},
		{"brotli", rpcResult(result), "br", nil, `unsupported Content-Encoding "br"`},
		{"gzip header on plain body", rpcResult(result), "gzip", nil, "gzip: invalid header"},
		{"not json", []byte("<html>"), "", nil, "invalid character"},
		{"rpc error", []byte(rpcBlockNotFound), "", nil, "rpc error 24: Block not found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := decodeTraces(test.body, test.encoding)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestTraceDecode(t *testing.T) {
	block := fixtureBlock(t, fixtureFrom)
	traces := rpcResult(syntheticTraces(t, block))
	otherTraces := rpcResult(syntheticTraces(t, fixtureBlock(t, fixtureFrom+1)))
	stateUpdateFile := mustResource(t, stateUpdate, blockKey{BlockNumber: fixtureFrom}).file
	stateUpdateBody := gzipped(t, readFixture(t, "state_update_with_block", "56377.json"))
	query := mustResource(t, preConfirmedBlock, blockKey{BlockNumber: fixtureFrom}).query

	tests := []struct {
		name        string
		stateUpdate []byte
		query       url.Values
		body        []byte
		encoding    string
		wantErr     string
		wantErrIs   error
	}{
		{name: "round built", stateUpdate: stateUpdateBody, query: query, body: traces},
		{
			name: "gzip response", stateUpdate: stateUpdateBody, query: query,
			body: gzipped(t, traces), encoding: "gzip",
		},
		{
			name: "malformed query", stateUpdate: stateUpdateBody,
			query: url.Values{"blockNumber": {"x"}}, body: traces,
			wantErr: "get_preconfirmed_block: ",
		},
		{name: "state update not captured", query: query, body: traces, wantErrIs: fs.ErrNotExist},
		{
			name: "state update lacks block", stateUpdate: gzipped(t, []byte(`{"state_update": {}}`)),
			query: query, body: traces, wantErr: "state update response lacks block",
		},
		{
			name: "rpc error", stateUpdate: stateUpdateBody, query: query, body: []byte(rpcBlockNotFound),
			wantErr: "tracing block 56377: rpc error 24: Block not found",
		},
		{
			name: "traces for another block", stateUpdate: stateUpdateBody, query: query, body: otherTraces,
			wantErr: "block 56377: trace has 51 transactions, block has 45",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataset := dataset{root: t.TempDir()}
			if test.stateUpdate != nil {
				require.NoError(t, dataset.write(stateUpdateFile, test.stateUpdate))
			}

			got, err := (&traceAPI{}).decode(dataset, resource{query: test.query}, test.body, test.encoding)
			switch {
			case test.wantErrIs != nil:
				require.ErrorIs(t, err, test.wantErrIs)
				return
			case test.wantErr != "":
				require.ErrorContains(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			plain, err := gunzip(got)
			require.NoError(t, err)
			envelope, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(plain))
			require.NoError(t, err)
			preConfirmed, ok := envelope.Update.(starknet.PreConfirmedBlock)
			require.True(t, ok, "got %T", envelope.Update)
			require.Equal(t, block.Hash, preConfirmed.BlockIdentifier)
			require.Len(t, preConfirmed.Transactions, len(block.Transactions))
		})
	}
}

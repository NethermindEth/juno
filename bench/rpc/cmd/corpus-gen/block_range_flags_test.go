package main

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecutionRangeReservesHeadBlock(t *testing.T) {
	server := httptest.NewServer(newFakeNode())
	defer server.Close()

	err := corpusGenError(
		t, "estimateFee", "--block-end", strconv.Itoa(fakeLatestBlock), "--source-url", server.URL,
	)
	require.Contains(t, err, "--block-end (100) must be <= 99")
}

func TestExecutionRangeRejectsLatestBlockID(t *testing.T) {
	server := httptest.NewServer(newFakeNode())
	defer server.Close()

	err := corpusGenError(t, "call", "--block-id", "latest", "--source-url", server.URL)
	require.Contains(t, err, "--block-id latest is not supported")
}

func TestReadRangeKeepsHeadBlock(t *testing.T) {
	server := httptest.NewServer(newFakeNode())
	defer server.Close()

	corpus := runCorpusGen(
		t,
		"getBlockWithTxHashes",
		"--count", "1",
		"--block-start", strconv.Itoa(fakeLatestBlock),
		"--source-url", server.URL,
	)
	require.Len(t, corpus.Requests, 1)
}

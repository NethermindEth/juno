package main_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNestedResampleStopsAtAttemptLimit(t *testing.T) {
	node := newFakeNode()
	server := httptest.NewServer(node)
	defer server.Close()

	err := corpusGenError(t, "getStorageProof", "--count", "1", "--source-url", server.URL)

	require.Contains(t, err, "sample class hash: no candidate found after 100 attempts")
	require.Equal(t, 100, node.stateUpdatesAsked())
}

package gateway

import (
	"net/http"
	"testing"
)

// TestHandlers tests the handlers defined in the gateway API.
func TestHandlers(t *testing.T) {
	gw := newTestGateway(t)

	ts := newTestServer(t, gw.routes())
	defer ts.Close()

	tests := [...]struct {
		route    string
		wantCode int
	}{
		{
			"/nonexistent",
			http.StatusNotFound,
		},
		{
			// TODO: This should default to the latest block but it is
			// currently not possible to compose this query.
			"/v0/get_block",
			http.StatusNotImplemented,
		},
		{
			"/v0/get_block?malformed=-1",
			http.StatusBadRequest,
		},
		{
			"/v0/get_block?blockNumber=0",
			http.StatusOK,
		},
		{
			// Valid block number but that is unlikely to be reached (maximum
			// value one can represent with an unsigned 64-bit integer).
			"/v0/get_block?blockNumber=18446744073709551615",
			http.StatusNotFound,
		},
		{
			// Negative block number.
			"/v0/get_block?blockNumber=-1",
			http.StatusBadRequest,
		},
		{
			"/v0/get_block?blockHash=0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
			http.StatusOK,
		},
		{
			// Nonexistent block.
			"/v0/get_block?blockHash=0x0",
			http.StatusNotFound,
		},
		{
			// Out of range block hash.
			"/v0/get_block?blockHash=0x800000000000011000000000000000000000000000000000000000000000002",
			http.StatusBadRequest,
		},
		{
			// Missing 0x prefix in blockHash parameter.
			"/v0/get_block?blockHash=47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
			http.StatusBadRequest,
		},
		{
			// Both blockNumber and blockHash arguments provided.
			"/v0/get_block?blockNumber=0&blockHash=0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
			http.StatusBadRequest,
		},
		{
			"/v0/get_block_hash_by_id?blockId=0",
			http.StatusOK,
		},
		{
			// No arguments supplied.
			"/v0/get_block_hash_by_id",
			http.StatusBadRequest,
		},
		{
			// Negative block id (number).
			"/v0/get_block_hash_by_id?blockId=-1",
			http.StatusBadRequest,
		},
		{
			// Valid block id but that is unlikely to be reached (maximum
			// value one can represent with an unsigned 64-bit integer).
			"/v0/get_block_hash_by_id?blockId=18446744073709551615",
			http.StatusNotFound,
		},
		{
			// Invalid arguments supplied.
			"/v0/get_block_hash_by_id?nonexistentKey=0",
			http.StatusBadRequest,
		},
		{
			"/v0/get_block_id_by_hash?blockHash=0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
			http.StatusOK,
		},
		{
			// Missing arguments.
			"/v0/get_block_id_by_hash",
			http.StatusBadRequest,
		},
		{
			// Nonexistent block.
			"/v0/get_block_id_by_hash?blockHash=0x0",
			http.StatusNotFound,
		},
		{
			// Out of range block hash.
			"/v0/get_block_id_by_hash?blockHash=0x800000000000011000000000000000000000000000000000000000000000002",
			http.StatusBadRequest,
		},
		{
			"/v0/get_block_id_by_hash?nonexistentKey=0x0",
			http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run("route = "+test.route, func(t *testing.T) {
			gotCode, _ := ts.get(t, test.route)
			if gotCode != test.wantCode {
				t.Fatalf("%s status = %d, want %d", test.route, gotCode, test.wantCode)
			}

			// TODO: Check response body.
		})
	}
}

// TODO: Add POST method tests for routes that only accept GET methods.

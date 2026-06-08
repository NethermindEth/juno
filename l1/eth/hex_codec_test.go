package eth_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/stretchr/testify/assert"
)

// TestHexCodecs_ErrorPaths exercises the HexU64 / HexBytes error branches
// indirectly through Log, ensuring malformed wire input is rejected rather
// than producing zero values silently.
func TestHexCodecs_ErrorPaths(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		// Unquoted JSON number — UnmarshalJSON receives bytes without surrounding quotes.
		{"bad blockNumber not a string", `{"topics":[],"data":"0x","blockNumber":1234,"removed":false}`},
		{"bad blockNumber prefix", `{"topics":[],"data":"0x","blockNumber":"1234","removed":false}`},
		{"bad blockNumber empty", `{"topics":[],"data":"0x","blockNumber":"0x","removed":false}`},
		{
			"bad blockNumber overflow",
			`{"topics":[],"data":"0x","blockNumber":"0x10000000000000000","removed":false}`,
		},
		{"bad blockNumber hex", `{"topics":[],"data":"0x","blockNumber":"0xZZ","removed":false}`},
		// JSON-RPC "quantity" must be minimally encoded — leading zeros are invalid.
		{
			"bad blockNumber leading zero",
			`{"topics":[],"data":"0x","blockNumber":"0x01","removed":false}`,
		},
		{
			"bad blockNumber leading zeros",
			`{"topics":[],"data":"0x","blockNumber":"0x00","removed":false}`,
		},
		{"bad data prefix", `{"topics":[],"data":"00","blockNumber":"0x0","removed":false}`},
		{"bad data odd length", `{"topics":[],"data":"0x0","blockNumber":"0x0","removed":false}`},
		{"bad data hex", `{"topics":[],"data":"0xZZ","blockNumber":"0x0","removed":false}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var l eth.Log
			err := json.Unmarshal([]byte(tc.json), &l)
			assert.Error(t, err, "input: %s", tc.json)
		})
	}
}

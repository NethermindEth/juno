package internal

import "github.com/ethereum/go-ethereum/common/hexutil"

// Takes in a Starknet chain ID constant as a string (e.g. "SN_MAIN")
// and returns the Starknet chain ID as a hex string
func EncodeChainId(chain string) string {
		b := []byte(chain)
		return hexutil.Encode(b)
}

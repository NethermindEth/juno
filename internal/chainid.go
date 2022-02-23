package internal

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	log "github.com/sirupsen/logrus"
)

// EncodeChainId Takes in a StarkNet chain ID constant as a string (e.g. "SN_MAIN")
// and returns the StarkNet chain ID as a hex string
func EncodeChainId(chain string) string {
	log.WithField("Chain", chain).Debug("Encoding Chain ID")
	b := []byte(chain)
	return hexutil.Encode(b)
}

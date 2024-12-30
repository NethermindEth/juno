package hashstorage

import (
	_ "embed"

	"github.com/NethermindEth/juno/core/felt"
)

const first0132SepoliaBlock = 86311

//go:embed sepolia_block_hashes.bin
var sepoliaBlockHashes []byte

var SepoliaBlockHashesMap = make(map[uint64]*felt.Felt, first0132SepoliaBlock)

//nolint:gochecknoinits
func init() {
	var offset uint64
	for i := uint64(0); i < first0132SepoliaBlock; i++ {
		offset = i * 32
		SepoliaBlockHashesMap[i] = new(felt.Felt).SetBytes(sepoliaBlockHashes[offset : offset+32])
	}
}

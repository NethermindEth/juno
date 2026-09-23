package deprecated

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/bits-and-blooms/bloom/v3"
)

// DeprecatedBlockHeader is the pre-bloom-removal core.Header layout, which
// embedded the event bloom filter. The migrations in this package run before
// the migration that strips the bloom out of headers, so on-disk headers still
// carry it; migrations that read the bloom or rewrite a header must go through
// this type to preserve it. Field names and cbor tags must match the old
// core.Header exactly.
type DeprecatedBlockHeader struct {
	Hash             *felt.Felt
	ParentHash       *felt.Felt
	Number           uint64
	GlobalStateRoot  *felt.Felt
	SequencerAddress *felt.Felt
	TransactionCount uint64
	EventCount       uint64
	Timestamp        uint64
	ProtocolVersion  string
	EventsBloom      *bloom.BloomFilter
	L1GasPriceETH    *felt.Felt `cbor:"gasprice"`
	Signatures       [][]*felt.Felt
	L1GasPriceSTRK   *felt.Felt `cbor:"gaspricestrk"`
	L1DAMode         core.L1DAMode
	L1DataGasPrice   *core.GasPrice
	L2GasPrice       *core.GasPrice
}

func getDeprecatedBlockHeader(
	r db.KeyValueReader,
	blockNumber uint64,
) (*DeprecatedBlockHeader, error) {
	var header *DeprecatedBlockHeader
	err := r.Get(db.BlockHeaderByNumberKey(blockNumber), func(data []byte) error {
		return encoder.Unmarshal(data, &header)
	})
	return header, err
}

func writeDeprecatedBlockHeader(w db.KeyValueWriter, header *DeprecatedBlockHeader) error {
	data, err := encoder.Marshal(header)
	if err != nil {
		return err
	}
	return w.Put(db.BlockHeaderByNumberKey(header.Number), data)
}

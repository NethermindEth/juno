package block

import (
	"encoding/json"
	"math/big"
)

type BlockProps struct {
	BlockHash             big.Int
	BlockNumber           uint64
	ParentBlockHash       big.Int
	Status                string
	SequencerAddress      big.Int
	GlobalStateRoot       big.Int
	OldRoot               big.Int
	AcceptedTime          int64
	BlockTimestamp        int64
	TransactionCount      int64
	TransactionCommitment big.Int
	EventCount            int64
	EventCommitment       big.Int
}

// BlockData is the struct used to store the data about a block
type BlockData struct {
	BlockProps
	TransactionsHashes []big.Int
}

func (x *BlockData) Marshal() ([]byte, error) {
	return json.Marshal(x)
}

func (x *BlockData) Unmarshal(data []byte) error {
	return json.Unmarshal(data, x)
}

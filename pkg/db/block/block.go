package block

import (
	"encoding/json"
	"fmt"
	"math/big"
)

// BlockHashKey is the key for the block database, in the base, it is the hash of the block itself.
type BlockHashKey string

func (x BlockHashKey) Marshal() ([]byte, error) {
	y, ok := new(big.Int).SetString(string(x), 16)
	if !ok {
		return nil, fmt.Errorf("invalid block hash key %s", x)
	}
	return y.Bytes(), nil
}

type BlockProps struct {
	BlockHash             string
	BlockNumber           uint64
	ParentBlockHash       string
	Status                string
	Sequencer             string
	GlobalStateRoot       string
	OldRoot               string
	AcceptedTime          int64
	BlockTimestamp        int64
	TransactionCount      int64
	TransactionCommitment string
	EventCount            int64
	EventCommitment       string
}

// BlockValue is the struct used to store the data about a block
type BlockValue struct {
	BlockProps
	TransactionsHashes []string
}

func (x *BlockValue) Marshal() ([]byte, error) {
	return json.Marshal(x)
}

func (x *BlockValue) Unmarshal(data []byte) error {
	return json.Unmarshal(data, x)
}

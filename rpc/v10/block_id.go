package rpcv10

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

// BlockStatus represents the status of a block.
type BlockStatus uint8

const (
	BlockPreConfirmed BlockStatus = iota
	BlockAcceptedL2
	BlockAcceptedL1
	BlockRejected
)

func (s BlockStatus) MarshalText() ([]byte, error) {
	switch s {
	case BlockPreConfirmed:
		return []byte("PRE_CONFIRMED"), nil
	case BlockAcceptedL2:
		return []byte("ACCEPTED_ON_L2"), nil
	case BlockAcceptedL1:
		return []byte("ACCEPTED_ON_L1"), nil
	case BlockRejected:
		return []byte("REJECTED"), nil
	default:
		return nil, fmt.Errorf("unknown block status %v", s)
	}
}

type blockIDType uint8

const (
	preConfirmed blockIDType = iota + 1
	latest
	hash
	number
	l1Accepted
)

func (b *blockIDType) String() string {
	switch *b {
	case preConfirmed:
		return "pre_confirmed"
	case latest:
		return "latest"
	case l1Accepted:
		return "l1_accepted"
	case hash:
		return "hash"
	case number:
		return "number"
	default:
		panic(fmt.Sprintf("Unknown blockIdType: %d", b))
	}
}

// BlockID represents a block identifier (by hash, number, or tag).
type BlockID struct {
	typeID blockIDType
	data   felt.Felt
}

func BlockIDFromNumber(num uint64) BlockID {
	return BlockID{
		typeID: number,
		data:   felt.Felt([4]uint64{num, 0, 0, 0}),
	}
}

func BlockIDFromHash(blockHash *felt.Felt) BlockID {
	return BlockID{
		typeID: hash,
		data:   *blockHash,
	}
}

func BlockIDPreConfirmed() BlockID {
	return BlockID{
		typeID: preConfirmed,
	}
}

func BlockIDLatest() BlockID {
	return BlockID{
		typeID: latest,
	}
}

func BlockIDL1Accepted() BlockID {
	return BlockID{
		typeID: l1Accepted,
	}
}

func (b *BlockID) Type() blockIDType {
	return b.typeID
}

func (b *BlockID) IsPreConfirmed() bool {
	return b.typeID == preConfirmed
}

func (b *BlockID) IsLatest() bool {
	return b.typeID == latest
}

func (b *BlockID) IsHash() bool {
	return b.typeID == hash
}

func (b *BlockID) IsNumber() bool {
	return b.typeID == number
}

func (b *BlockID) IsL1Accepted() bool {
	return b.typeID == l1Accepted
}

func (b *BlockID) Hash() *felt.Felt {
	if b.typeID != hash {
		panic(fmt.Sprintf("Trying to get hash from block id with type %s", b.typeID.String()))
	}
	return &b.data
}

func (b *BlockID) Number() uint64 {
	if b.typeID != number {
		panic(fmt.Sprintf("Trying to get number from block id with type %s", b.typeID.String()))
	}
	return b.data[0]
}

func (b *BlockID) UnmarshalJSON(data []byte) error {
	var blockTag string
	if err := json.Unmarshal(data, &blockTag); err == nil {
		switch blockTag {
		case "latest":
			b.typeID = latest
		case "pre_confirmed":
			b.typeID = preConfirmed
		case "l1_accepted":
			b.typeID = l1Accepted
		default:
			return fmt.Errorf("unknown block tag '%s'", blockTag)
		}
	} else {
		jsonObject := make(map[string]json.RawMessage)
		if err := json.Unmarshal(data, &jsonObject); err != nil {
			return err
		}
		blockHash, ok := jsonObject["block_hash"]
		if ok {
			b.typeID = hash
			return json.Unmarshal(blockHash, &b.data)
		}

		blockNumber, ok := jsonObject["block_number"]
		if ok {
			b.typeID = number
			return json.Unmarshal(blockNumber, &b.data[0])
		}

		return errors.New("cannot unmarshal block id")
	}
	return nil
}

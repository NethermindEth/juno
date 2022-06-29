package starknet

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/pkg/common"
	"github.com/NethermindEth/juno/pkg/types"
)

type BlockTag string

const (
	BlocktagLatest  BlockTag = "latest"
	BlocktagPending BlockTag = "pending"
)

var blockTags = []BlockTag{BlocktagLatest, BlocktagPending}

type BlockHashOrTag struct {
	Hash *types.BlockHash
	Tag  *BlockTag
}

func (x *BlockHashOrTag) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewBuffer(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch t := token.(type) {
	case string:
		for _, tag := range blockTags {
			if t == string(tag) {
				*x = BlockHashOrTag{}
				x.Tag = &tag
				return nil
			}
		}
		if common.IsHex(t) {
			hash := types.HexToBlockHash(t)
			*x = BlockHashOrTag{
				Hash: &hash,
			}
		}
	default:
		// notest
		return errors.New("unexpected token type")
	}
	return nil
}

type RequestedScope string

const (
	ScopeTxnHash            RequestedScope = "TXN_HASH"
	ScopeFullTxns           RequestedScope = "FULL_TXNS"
	ScopeFullTxnAndReceipts RequestedScope = "FULL_TXN_AND_RECEIPTS"
)

type ProtocolVersion string

type BlockNumberOrTag struct {
	Number *uint64
	Tag    *BlockTag
}

func (x *BlockNumberOrTag) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewBuffer(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch t := token.(type) {
	case float64:
		blockNumber := uint64(t)
		if blockNumber < 0 {
			// notest
			return errors.New("invalid block number")
		}
		*x = BlockNumberOrTag{Number: &blockNumber}
	case string:
		// notest
		for _, tag := range blockTags {
			if t == string(tag) {
				*x = BlockNumberOrTag{Tag: &tag}
				break
			}
		}
	default:
		// notest
		return errors.New("unexpected token type")
	}
	return nil
}


package rpc

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

// https://github.com/starkware-libs/starknet-specs/blob/fbf8710c2d2dcdb70a95776f257d080392ad0816/api/starknet_api_openrpc.json#L2353-L2363
type BlockStatus uint8

const (
	BlockPending BlockStatus = iota
	BlockAcceptedL2
	BlockAcceptedL1
	BlockRejected
)

func (s BlockStatus) MarshalText() ([]byte, error) {
	switch s {
	case BlockPending:
		return []byte("PENDING"), nil
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

type L1DAMode uint8

const (
	Blob L1DAMode = iota
	Calldata
)

func (l L1DAMode) MarshalText() ([]byte, error) {
	switch l {
	case Blob:
		return []byte("BLOB"), nil
	case Calldata:
		return []byte("CALLDATA"), nil
	default:
		return nil, fmt.Errorf("unknown L1DAMode value = %v", l)
	}
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L520-L534
type BlockHashAndNumber struct {
	Hash   *felt.Felt `json:"block_hash"`
	Number uint64     `json:"block_number"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L814
type BlockID struct {
	Pending bool
	Latest  bool
	Hash    *felt.Felt
	Number  uint64
}

func (b *BlockID) UnmarshalJSON(data []byte) error {
	if string(data) == `"latest"` {
		b.Latest = true
	} else if string(data) == `"pending"` {
		b.Pending = true
	} else {
		jsonObject := make(map[string]json.RawMessage)
		if err := json.Unmarshal(data, &jsonObject); err != nil {
			return err
		}
		hash, ok := jsonObject["block_hash"]
		if ok {
			b.Hash = new(felt.Felt)
			return json.Unmarshal(hash, b.Hash)
		}

		number, ok := jsonObject["block_number"]
		if ok {
			return json.Unmarshal(number, &b.Number)
		}

		return errors.New("cannot unmarshal block id")
	}
	return nil
}

type ResourcePrice struct {
	InFri *felt.Felt `json:"price_in_fri"`
	InWei *felt.Felt `json:"price_in_wei"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1072
type BlockHeader struct {
	Hash             *felt.Felt     `json:"block_hash,omitempty"`
	ParentHash       *felt.Felt     `json:"parent_hash"`
	Number           *uint64        `json:"block_number,omitempty"`
	NewRoot          *felt.Felt     `json:"new_root,omitempty"`
	Timestamp        uint64         `json:"timestamp"`
	SequencerAddress *felt.Felt     `json:"sequencer_address,omitempty"`
	L1GasPrice       *ResourcePrice `json:"l1_gas_price"`
	L1DataGasPrice   *ResourcePrice `json:"l1_data_gas_price,omitempty"`
	L1DAMode         *L1DAMode      `json:"l1_da_mode,omitempty"`
	StarknetVersion  string         `json:"starknet_version"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1131
type BlockWithTxs struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []*Transaction `json:"transactions"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1109
type BlockWithTxHashes struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	TxnHashes []*felt.Felt `json:"transactions"`
}

type TransactionWithReceipt struct {
	Transaction *Transaction        `json:"transaction"`
	Receipt     *TransactionReceipt `json:"receipt"`
}

type BlockWithReceipts struct {
	Status BlockStatus `json:"status,omitempty"`
	BlockHeader
	Transactions []TransactionWithReceipt `json:"transactions"`
}

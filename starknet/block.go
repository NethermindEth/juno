package starknet

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

// Block object returned by the feeder in JSON format for "get_block" endpoint
type Block struct {
	Hash                  *felt.Felt            `json:"block_hash"`
	ParentHash            *felt.Felt            `json:"parent_block_hash"`
	Number                uint64                `json:"block_number"`
	StateRoot             *felt.Felt            `json:"state_root"`
	TransactionCommitment *felt.Felt            `json:"transaction_commitment"`
	EventCommitment       *felt.Felt            `json:"event_commitment"`
	ReceiptCommitment     *felt.Felt            `json:"receipt_commitment"`
	StateDiffCommitment   *felt.Felt            `json:"state_diff_commitment"`
	StateDiffLength       uint64                `json:"state_diff_length"`
	Status                string                `json:"status"`
	Transactions          []*Transaction        `json:"transactions"`
	Timestamp             uint64                `json:"timestamp"`
	Version               string                `json:"starknet_version"`
	Receipts              []*TransactionReceipt `json:"transaction_receipts"`
	SequencerAddress      *felt.Felt            `json:"sequencer_address"`
	L1GasPrice            *GasPrice             `json:"l1_gas_price"`
	L1DAMode              L1DAMode              `json:"l1_da_mode"`
	L1DataGasPrice        *GasPrice             `json:"l1_data_gas_price"`
}

func (b *Block) GasPriceETH() *felt.Felt {
	if b.L1GasPrice == nil {
		fmt.Println("Block is ", b.Number)
	}
	return b.L1GasPrice.PriceInWei
}

func (b *Block) GasPriceSTRK() *felt.Felt {
	return b.L1GasPrice.PriceInFri
}

type L1DAMode uint

const (
	Calldata L1DAMode = iota
	Blob
)

func (m *L1DAMode) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"CALLDATA"`:
		*m = Calldata
	case `"BLOB"`:
		*m = Blob
	default:
		return errors.New("unknown L1DAMode")
	}
	return nil
}

type GasPrice struct {
	PriceInWei *felt.Felt `json:"price_in_wei"`
	PriceInFri *felt.Felt `json:"price_in_fri"`
}

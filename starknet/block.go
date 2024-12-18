package starknet

import (
	"errors"

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
	L2GasPrice            *GasPrice             `json:"l2_gas_price"`
	L1DAMode              L1DAMode              `json:"l1_da_mode"`
	L1DataGasPrice        *GasPrice             `json:"l1_data_gas_price"`
	L2GasPrice            *GasPrice             `json:"l2_gas_price"`

	// TODO we can remove the GasPrice method and the GasPriceLegacy field
	// once v0.13 lands on mainnet. In the meantime, we include both to support
	// pre-v0.13 jsons, where `eth_l1_gas_price` was called `gas_price`.
	GasPriceLegacy *felt.Felt `json:"gas_price"`
	// TODO these fields were replaced by `l1_gas_price` in v0.13.1
	GasPriceWEI *felt.Felt `json:"eth_l1_gas_price"`
	GasPriceFRI *felt.Felt `json:"strk_l1_gas_price"`
}

func (b *Block) L1GasPriceETH() *felt.Felt {
	if b.L1GasPrice != nil {
		return b.L1GasPrice.PriceInWei
	} else if b.GasPriceWEI != nil {
		return b.GasPriceWEI
	}
	return b.GasPriceLegacy
}

func (b *Block) L1GasPriceSTRK() *felt.Felt {
	if b.L1GasPrice != nil {
		return b.L1GasPrice.PriceInFri
	}
	return b.GasPriceFRI
}

// TODO: Fix when we have l2 gas price
func (b *Block) L2GasPriceETH() *felt.Felt {
	if b.L2GasPrice != nil {
		return b.L2GasPrice.PriceInWei
	}
	return &felt.Zero
}

// TODO: Fix when we have l2 gas price
func (b *Block) L2GasPriceSTRK() *felt.Felt {
	if b.L2GasPrice != nil {
		return b.L2GasPrice.PriceInFri
	}
	return &felt.Zero
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

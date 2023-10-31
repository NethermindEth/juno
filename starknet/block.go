package starknet

import "github.com/NethermindEth/juno/core/felt"

// Block object returned by the feeder in JSON format for "get_block" endpoint
type Block struct {
	Hash             *felt.Felt            `json:"block_hash"`
	ParentHash       *felt.Felt            `json:"parent_block_hash"`
	Number           uint64                `json:"block_number"`
	StateRoot        *felt.Felt            `json:"state_root"`
	Status           string                `json:"status"`
	GasPrice         *felt.Felt            `json:"gas_price"`
	Transactions     []*Transaction        `json:"transactions"`
	Timestamp        uint64                `json:"timestamp"`
	Version          string                `json:"starknet_version"`
	Receipts         []*TransactionReceipt `json:"transaction_receipts"`
	SequencerAddress *felt.Felt            `json:"sequencer_address"`
}

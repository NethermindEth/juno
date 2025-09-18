package starknet

import "github.com/NethermindEth/juno/core/types/felt"

// Block object returned by the feeder in JSON format for "get_preconfirmed_block" endpoint
type PreConfirmedBlock struct {
	Status           string                `json:"status"`
	Transactions     []Transaction         `json:"transactions"`
	Timestamp        uint64                `json:"timestamp"`
	Version          string                `json:"starknet_version"`
	Receipts         []*TransactionReceipt `json:"transaction_receipts"`
	SequencerAddress *felt.Felt            `json:"sequencer_address"`
	L1GasPrice       *GasPrice             `json:"l1_gas_price"`
	L2GasPrice       *GasPrice             `json:"l2_gas_price"`
	L1DAMode         L1DAMode              `json:"l1_da_mode"`
	L1DataGasPrice   *GasPrice             `json:"l1_data_gas_price"`

	TransactionStateDiffs []*StateDiff `json:"transaction_state_diffs"`
}

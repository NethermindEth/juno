package starknet

import "github.com/NethermindEth/juno/core/felt"

// PreConfirmedBlock is the response object returned by the feeder for
// "get_preconfirmed_block". It supports three response modes negotiated via the
// blockIdentifier and knownTransactionCount query parameters: no-change,
// delta and full. Use Mode to discriminate.
type PreConfirmedBlock struct {
	// Present on all responses.
	Changed bool `json:"changed"`

	// Present on full and delta responses.
	BlockIdentifier       string                `json:"block_identifier,omitempty"`
	Transactions          []Transaction         `json:"transactions,omitempty"`
	Receipts              []*TransactionReceipt `json:"transaction_receipts,omitempty"`
	TransactionStateDiffs []*StateDiff          `json:"transaction_state_diffs,omitempty"`

	// Present only on full responses.
	Status           string     `json:"status,omitempty"`
	Timestamp        uint64     `json:"timestamp,omitempty"`
	Version          string     `json:"starknet_version,omitempty"`
	SequencerAddress *felt.Felt `json:"sequencer_address,omitempty"`
	L1GasPrice       *GasPrice  `json:"l1_gas_price,omitempty"`
	L2GasPrice       *GasPrice  `json:"l2_gas_price,omitempty"`
	L1DAMode         L1DAMode   `json:"l1_da_mode,omitempty"`
	L1DataGasPrice   *GasPrice  `json:"l1_data_gas_price,omitempty"`
}

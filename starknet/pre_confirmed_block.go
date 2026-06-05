package starknet

import "github.com/NethermindEth/juno/core/felt"

// DeprecatedPreConfirmedBlock is the response object returned by the legacy
// "get_preconfirmed_block" endpoint (no blockIdentifier/knownTransactionCount
// parameters). For the delta-aware endpoint, use [PreConfirmedUpdate] via
// [PreConfirmedUpdateEnvelope].
//
// Deprecated: prefer [PreConfirmedUpdate]; this type only exists to support
// the legacy "get_preconfirmed_block" endpoint until it can be removed.
type DeprecatedPreConfirmedBlock struct {
	Transactions          []Transaction         `json:"transactions,omitempty"`
	Receipts              []*TransactionReceipt `json:"transaction_receipts,omitempty"`
	TransactionStateDiffs []*StateDiff          `json:"transaction_state_diffs,omitempty"`

	Status           string     `json:"status,omitempty"`
	Timestamp        uint64     `json:"timestamp,omitempty"`
	Version          string     `json:"starknet_version,omitempty"`
	SequencerAddress *felt.Felt `json:"sequencer_address,omitempty"`
	L1GasPrice       *GasPrice  `json:"l1_gas_price,omitempty"`
	L2GasPrice       *GasPrice  `json:"l2_gas_price,omitempty"`
	L1DAMode         L1DAMode   `json:"l1_da_mode,omitempty"`
	L1DataGasPrice   *GasPrice  `json:"l1_data_gas_price,omitempty"`
}

// AsUpdate adapts a legacy DeprecatedPreConfirmedBlock to the new
// [PreConfirmedBlock] variant for the unified ApplyUpdate path. Slice headers
// are shared (no copy).
func (b *DeprecatedPreConfirmedBlock) AsUpdate() PreConfirmedUpdate {
	return PreConfirmedBlock{
		BlockIdentifier:       "LegacyAPI",
		Transactions:          b.Transactions,
		Receipts:              b.Receipts,
		TransactionStateDiffs: b.TransactionStateDiffs,
		Status:                b.Status,
		Timestamp:             b.Timestamp,
		Version:               b.Version,
		SequencerAddress:      b.SequencerAddress,
		L1GasPrice:            b.L1GasPrice,
		L2GasPrice:            b.L2GasPrice,
		L1DAMode:              b.L1DAMode,
		L1DataGasPrice:        b.L1DataGasPrice,
	}
}

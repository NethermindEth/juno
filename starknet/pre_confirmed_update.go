package starknet

import (
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
)

// PreConfirmedUpdate is a sealed sum type for "get_preconfirmed_block" responses.
// One of [PreConfirmedNoChange], [PreConfirmedDeltaUpdate], or [PreConfirmedBlock].
type PreConfirmedUpdate interface {
	isPreConfirmedUpdate()
}

// PreConfirmedNoChange means the server's pre_confirmed matches what the caller already has.
//
// Note: the wire JSON also carries a top-level `"changed"` boolean which is not
// modelled here. [PreConfirmedUpdateEnvelope.UnmarshalJSON] peeks at it during
// variant discrimination and discards it before decoding into this struct.
type PreConfirmedNoChange struct{}

// PreConfirmedDeltaUpdate carries transactions/receipts/state diffs appended since the
// caller's known transaction count for the same block_identifier.
//
// Note: the wire JSON also carries a top-level `"changed"` boolean which is not
// modelled here. [PreConfirmedUpdateEnvelope.UnmarshalJSON] peeks at it during
// variant discrimination and discards it before decoding into this struct.
type PreConfirmedDeltaUpdate struct {
	BlockIdentifier       string                `json:"block_identifier"`
	Transactions          []Transaction         `json:"transactions"`
	Receipts              []*TransactionReceipt `json:"transaction_receipts"`
	TransactionStateDiffs []*StateDiff          `json:"transaction_state_diffs"`
}

// PreConfirmedBlock carries a full pre_confirmed block for a new round.
//
// Note: the wire JSON also carries a top-level `"changed"` boolean which is not
// modelled here. [PreConfirmedUpdateEnvelope.UnmarshalJSON] peeks at it during
// variant discrimination and discards it before decoding into this struct.
type PreConfirmedBlock struct {
	BlockIdentifier       string                `json:"block_identifier"`
	Transactions          []Transaction         `json:"transactions"`
	Receipts              []*TransactionReceipt `json:"transaction_receipts"`
	TransactionStateDiffs []*StateDiff          `json:"transaction_state_diffs"`
	Status                string                `json:"status"`
	Timestamp             uint64                `json:"timestamp"`
	Version               string                `json:"starknet_version"`
	SequencerAddress      *felt.Felt            `json:"sequencer_address"`
	L1GasPrice            *GasPrice             `json:"l1_gas_price"`
	L2GasPrice            *GasPrice             `json:"l2_gas_price"`
	L1DAMode              L1DAMode              `json:"l1_da_mode"`
	L1DataGasPrice        *GasPrice             `json:"l1_data_gas_price"`
}

func (PreConfirmedNoChange) isPreConfirmedUpdate()    {}
func (PreConfirmedDeltaUpdate) isPreConfirmedUpdate() {}
func (PreConfirmedBlock) isPreConfirmedUpdate()       {}

var (
	_ PreConfirmedUpdate = PreConfirmedNoChange{}
	_ PreConfirmedUpdate = PreConfirmedDeltaUpdate{}
	_ PreConfirmedUpdate = PreConfirmedBlock{}
)

// PreConfirmedUpdateEnvelope is the JSON-decodable carrier for a [PreConfirmedUpdate].
// Discrimination is structural:
//   - "changed": false                → NoChange
//   - "changed": true + "timestamp"   → Full (new round)
//   - "changed": true, no "timestamp" → Delta
type PreConfirmedUpdateEnvelope struct {
	Update PreConfirmedUpdate
}

func (e *PreConfirmedUpdateEnvelope) UnmarshalJSON(data []byte) error {
	var peek struct {
		Changed   *bool   `json:"changed"`
		Timestamp *uint64 `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return err
	}
	if peek.Changed == nil {
		return errors.New("pre_confirmed update: missing required \"changed\" field")
	}

	switch {
	case !*peek.Changed:
		e.Update = PreConfirmedNoChange{}
	case peek.Timestamp != nil:
		var full PreConfirmedBlock
		if err := json.Unmarshal(data, &full); err != nil {
			return err
		}
		e.Update = full
	default:
		var delta PreConfirmedDeltaUpdate
		if err := json.Unmarshal(data, &delta); err != nil {
			return err
		}
		e.Update = delta
	}
	return nil
}

// TODO: placeholder for now to avoid compiler errors. A proper validation
// should be implemented in a follow-up PR.
func (val *Block) Validate() (*Block, error) {
	return val, nil
}
func (val *BlockHeader) Validate() (*BlockHeader, error) {
	return val, nil
}
func (val *BlockTrace) Validate() (*BlockTrace, error) {
	return val, nil
}
func (val *ClassDefinition) Validate() (*ClassDefinition, error) {
	return val, nil
}
func (val *FeeTokenAddresses) Validate() (*FeeTokenAddresses, error) {
	return val, nil
}
func (val *Signature) Validate() (*Signature, error) {
	return val, nil
}
func (val *StateUpdate) Validate() (*StateUpdate, error) {
	return val, nil
}
func (val *StateUpdateWithBlock) Validate() (*StateUpdateWithBlock, error) {
	return val, nil
}
func (val *StateUpdateWithBlockAndSignature) Validate() (*StateUpdateWithBlockAndSignature, error) {
	return val, nil
}
func (val *DeprecatedPreConfirmedBlock) Validate() (*DeprecatedPreConfirmedBlock, error) {
	return val, nil
}
func (val *PreConfirmedUpdateEnvelope) Validate() (*PreConfirmedUpdateEnvelope, error) {
	return val, nil
}
func (val *DeprecatedTransactionStatus) Validate() (*DeprecatedTransactionStatus, error) {
	return val, nil
}
func (val *TransactionStatus) Validate() (*TransactionStatus, error) {
	return val, nil
}

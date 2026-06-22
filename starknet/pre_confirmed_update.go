package starknet

import (
	"encoding/json"
	"errors"
	"fmt"

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

func (PreConfirmedNoChange) isPreConfirmedUpdate() {}

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

func (PreConfirmedDeltaUpdate) isPreConfirmedUpdate() {}

func (val *PreConfirmedDeltaUpdate) validate() error {
	if val.BlockIdentifier == "" {
		return errors.New("block_identifier is required")
	}
	if len(val.Transactions) == 0 {
		return errors.New("delta can not have zero transactions")
	}

	return validateTxsLength(
		val.Transactions, val.Receipts, val.TransactionStateDiffs,
	)
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

func (PreConfirmedBlock) isPreConfirmedUpdate() {}

func (pb *PreConfirmedBlock) validate() error {
	if pb.BlockIdentifier == "" {
		return errors.New("block_identifier is required")
	}
	if pb.Status != "PRE_CONFIRMED" {
		return fmt.Errorf("invalid status: %s", pb.Status)
	}
	if pb.Version == "" {
		return errors.New("version is required")
	}
	if pb.SequencerAddress == nil {
		return errors.New("sequencer_address is required")
	}
	if pb.L1GasPrice == nil {
		return errors.New("l1_gas_price is required")
	}
	if pb.L2GasPrice == nil {
		return errors.New("l2_gas_price is required")
	}
	if pb.L1DataGasPrice == nil {
		return errors.New("l1_data_gas_price is required")
	}
	return validateTxsLength(
		pb.Transactions, pb.Receipts, pb.TransactionStateDiffs,
	)
}

func validateTxsLength(
	txs []Transaction,
	receipts []*TransactionReceipt,
	stateDiffs []*StateDiff,
) error {
	if len(txs) != len(receipts) ||
		len(txs) != len(stateDiffs) {
		return errors.New(
			"transactions, receipts, and tx_state_diffs must have the same length",
		)
	}

	for i := range txs {
		if txs[i] == (Transaction{}) {
			return fmt.Errorf("transaction at index %d is empty", i)
		}
		if receipts[i] == nil {
			return fmt.Errorf("receipt at index %d is nil", i)
		}
		if stateDiffs[i] == nil {
			return fmt.Errorf("transaction state diff at index %d is nil", i)
		}
	}
	return nil
}

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

func (e *PreConfirmedUpdateEnvelope) Validate() (*PreConfirmedUpdateEnvelope, error) {
	switch update := e.Update.(type) {
	case PreConfirmedNoChange:
	case PreConfirmedDeltaUpdate:
		if err := update.validate(); err != nil {
			return nil, err
		}
	case PreConfirmedBlock:
		if err := update.validate(); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid pre_confirmed update type %T", e.Update)
	}
	return e, nil
}

// TODO: placeholder for now to avoid compiler errors. A proper validation
// should be implemented in a follow-up PR.

func (b *Block) Validate() (*Block, error) {
	return b, nil
}

func (val *BlockHeader) Validate() (*BlockHeader, error) {
	return val, nil
}

func (val *BlockTrace) Validate() (*BlockTrace, error) {
	return val, nil
}

func (c *ClassDefinition) Validate() (*ClassDefinition, error) {
	return c, nil
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

func (b *DeprecatedPreConfirmedBlock) Validate() (*DeprecatedPreConfirmedBlock, error) {
	return b, nil
}

func (val *DeprecatedTransactionStatus) Validate() (*DeprecatedTransactionStatus, error) {
	return val, nil
}

func (val *TransactionStatus) Validate() (*TransactionStatus, error) {
	return val, nil
}

package starknet

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NethermindEth/juno/core/felt"
)

// PreConfirmedUpdate is a sealed sum type for "get_preconfirmed_block" responses.
// One of [PreConfirmedNoChange], [PreConfirmedDeltaUpdate], or [PreConfirmedBlock].
type PreConfirmedUpdate interface {
	isPreConfirmedUpdate()
}

type BlockIdentifier uint64

func (b *BlockIdentifier) UnmarshalJSON(data []byte) error {
	// Handle quoted string like "0x1676efd0"
	if len(data) > 0 && data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		str = strings.TrimPrefix(str, "0x")
		val, err := strconv.ParseUint(str, 16, 64)
		if err != nil {
			return err
		}
		*b = BlockIdentifier(val)
		return nil
	}
	// Handle bare integer like 123
	var val uint64
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}
	*b = BlockIdentifier(val)
	return nil
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
	BlockIdentifier       BlockIdentifier       `json:"block_identifier"`
	Transactions          []Transaction         `json:"transactions"`
	Receipts              []*TransactionReceipt `json:"transaction_receipts"`
	TransactionStateDiffs []*StateDiff          `json:"transaction_state_diffs"`
}

func (PreConfirmedDeltaUpdate) isPreConfirmedUpdate() {}

func (val *PreConfirmedDeltaUpdate) validate() error {
	if val.BlockIdentifier == 0 {
		return errors.New("block_identifier is required")
	}
	if len(val.Transactions) == 0 {
		return errors.New("delta requires at least one transaction")
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
	BlockIdentifier       BlockIdentifier       `json:"block_identifier"`
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
	if pb.BlockIdentifier == 0 {
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
	if len(txs) != len(receipts) || len(txs) != len(stateDiffs) {
		return fmt.Errorf(
			"transactions, receipts, and tx_state_diffs must have the same length: "+
				"txs: %d, receipts: %d, stateDiffs: %d",
			len(txs),
			len(receipts),
			len(stateDiffs),
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

func (e *PreConfirmedUpdateEnvelope) Validate() error {
	switch update := e.Update.(type) {
	case PreConfirmedNoChange:
	case PreConfirmedDeltaUpdate:
		if err := update.validate(); err != nil {
			return fmt.Errorf("invalid pre_confirmed delta update: %w", err)
		}
	case PreConfirmedBlock:
		if err := update.validate(); err != nil {
			return fmt.Errorf("invalid pre_confirmed full block: %w", err)
		}
	default:
		return fmt.Errorf("invalid pre_confirmed update type %T", e.Update)
	}
	return nil
}

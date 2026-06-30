package starknet

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/NethermindEth/juno/core/felt"
)

// PreConfirmedUpdate is a sealed sum type for "get_preconfirmed_block" responses:
// one of [PreConfirmedNoChange], [PreConfirmedDeltaUpdate], or [PreConfirmedBlock].
//
// The wire JSON's top-level "changed" boolean is not modelled on any variant;
// [DecodePreConfirmedUpdate] uses it only to discriminate, then discards it.
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
type PreConfirmedNoChange struct{}

func (PreConfirmedNoChange) isPreConfirmedUpdate() {}

// PreConfirmedDeltaUpdate carries transactions/receipts/state diffs appended since the
// caller's known transaction count for the same block_identifier.
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
	if pb.Timestamp == 0 {
		return errors.New("timestamp is required")
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

// PreConfirmedUpdateEnvelope is the discriminated carrier for a [PreConfirmedUpdate].
//
// BlockNumber is set when the response carries a top-level "block_number": the
// "latest" endpoint includes it; the explicit-number endpoint does not, since
// the caller already knows the requested number.
//
// Decode via [DecodePreConfirmedUpdate].
type PreConfirmedUpdateEnvelope struct {
	Update      PreConfirmedUpdate
	BlockNumber uint64
}

// DecodePreConfirmedUpdate decodes a "get_preconfirmed_block" response and
// discriminates it into a [PreConfirmedUpdateEnvelope].
func DecodePreConfirmedUpdate(r io.Reader) (PreConfirmedUpdateEnvelope, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return PreConfirmedUpdateEnvelope{}, err
	}

	// The discriminator is decoded on its own first: a NoChange response is
	// identified purely by "changed": false, and any other field must be
	// ignored even if malformed, so the full payload is only decoded once
	// changed is known to be true.
	var discriminator struct {
		Changed     *bool   `json:"changed"`
		BlockNumber *uint64 `json:"block_number"`
	}
	if err := json.Unmarshal(data, &discriminator); err != nil {
		return PreConfirmedUpdateEnvelope{}, err
	}
	if discriminator.Changed == nil {
		return PreConfirmedUpdateEnvelope{}, errors.New(
			"missing required \"changed\" field",
		)
	}

	var env PreConfirmedUpdateEnvelope
	if discriminator.BlockNumber != nil {
		env.BlockNumber = *discriminator.BlockNumber
	}

	if !*discriminator.Changed {
		env.Update = PreConfirmedNoChange{}
		return env, nil
	}

	// Discrimination between Full (new round) and Delta is structural:
	//   - "timestamp" present → Full block (new round)
	//   - "timestamp" absent  → Delta
	var body struct {
		PreConfirmedBlock
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return PreConfirmedUpdateEnvelope{}, err
	}

	switch {
	case body.Timestamp != 0:
		env.Update = body.PreConfirmedBlock
	default:
		env.Update = PreConfirmedDeltaUpdate{
			BlockIdentifier:       body.BlockIdentifier,
			Transactions:          body.Transactions,
			Receipts:              body.Receipts,
			TransactionStateDiffs: body.TransactionStateDiffs,
		}
	}
	return env, nil
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

package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

const (
	invokeTxType        = "INVOKE"
	declareTxType       = "DECLARE"
	deployAccountTxType = "DEPLOY_ACCOUNT"
	deployTxType        = "DEPLOY"
	l1HandlerTxType     = "L1_HANDLER"
	broadcastTxVersion  = "0x3"

	defaultNumTxs = 1
)

type broadcastedTxsArgs struct {
	blockIDArgs
	MinTxs  uint64     `json:"minTxs"`
	MaxTxs  uint64     `json:"maxTxs"`
	TxTypes []string   `json:"txTypes"`
	Verify  verifyFlag `json:"verify"`
}

func (a *broadcastedTxsArgs) bind(cmd *cobra.Command, client *rpcClient) {
	var numTxs []uint
	cmd.Flags().UintSliceVar(
		&numTxs,
		"num-txs",
		[]uint{defaultNumTxs},
		"Broadcasted transactions per request: one value or min,max.",
	)
	cmd.Flags().StringSliceVar(
		&a.TxTypes,
		"tx-types",
		[]string{invokeTxType, deployAccountTxType},
		"Transaction types to take from the block: INVOKE, DECLARE or DEPLOY_ACCOUNT. "+
			"A transaction of any other type ends the run, so a narrow list draws more blocks. "+
			"DECLARE carries a full contract class, which the node compiles on every request.",
	)
	chainPreRunE(cmd, func() error {
		if err := normalizeTxTypes(a.TxTypes); err != nil {
			return err
		}
		var err error
		a.MinTxs, a.MaxTxs, err = parseBounds("num-txs", numTxs)
		return err
	})
	a.Verify.bind(cmd, client)
	bindExecutionRange(cmd, client, &a.blockIDArgs)
}

// normalizeTxTypes upper-cases the types in place, and rejects the ones the
// execution methods do not take.
func normalizeTxTypes(txTypes []string) error {
	for i, txType := range txTypes {
		txTypes[i] = strings.ToUpper(txType)
		switch txTypes[i] {
		case invokeTxType, declareTxType, deployAccountTxType:
		case deployTxType, l1HandlerTxType:
			return fmt.Errorf("--tx-types %s cannot be broadcast", txTypes[i])
		default:
			return fmt.Errorf(
				"--tx-types must be INVOKE, DECLARE or DEPLOY_ACCOUNT (got %q)", txType,
			)
		}
	}
	return nil
}

// sampleBroadcastedTxs takes a leading run of the transactions of block N+1 as
// a broadcast against block N, whose state they ran on; real transactions keep
// their nonces and signatures valid, so validation needs no skipping.
func sampleBroadcastedTxs(
	input samplerInput[broadcastedTxsArgs],
) ([]broadcastedTx, blockID, error) {
	args := input.args
	blockNumber := args.sampleBlockNumber(input.rng)
	want := int(uniformRange(input.rng, args.MinTxs, args.MaxTxs))

	next := blockNumber + 1
	block, err := input.client.blockWithTxs(input.ctx, next)
	if err != nil {
		return nil, nil, err
	}
	prefix := broadcastablePrefix(block.Transactions, args.TxTypes)
	if len(prefix) < want {
		return nil, nil, fmt.Errorf(
			"broadcastable prefix of block %d is %d long, want %d: %w",
			next, len(prefix), want, errResample,
		)
	}

	prefix = prefix[:want]
	for _, tx := range prefix {
		stripResponseFields(tx)
		if tx.text("type") != declareTxType {
			continue
		}
		class, classErr := input.client.rawClassAt(input.ctx, next, tx.text("class_hash"))
		if classErr != nil {
			return nil, nil, classErr
		}
		tx["contract_class"] = class
		delete(tx, "class_hash")
	}

	id, err := resolveBlockID(input.ctx, input.client, args.BlockIDKind, blockNumber)
	if err != nil {
		return nil, nil, err
	}
	return prefix, id, nil
}

// text returns the named field of tx, or "" when the field is absent.
func (tx broadcastedTx) text(field string) string {
	var value string
	if err := json.Unmarshal(tx[field], &value); err != nil {
		return ""
	}
	return value
}

// broadcastablePrefix returns the leading run of txs of the wanted types; a
// contiguous prefix keeps the nonce of every sender in order.
func broadcastablePrefix(txs []broadcastedTx, txTypes []string) []broadcastedTx {
	for i, tx := range txs {
		if !broadcastable(tx, txTypes) {
			return txs[:i]
		}
	}
	return txs
}

func broadcastable(tx broadcastedTx, txTypes []string) bool {
	return tx.text("version") == broadcastTxVersion && slices.Contains(txTypes, tx.text("type"))
}

// stripResponseFields removes the fields that responses carry but broadcasts
// do not; the node derives the deploy account address from the other fields.
func stripResponseFields(tx broadcastedTx) {
	delete(tx, "transaction_hash")
	delete(tx, "contract_address")
}

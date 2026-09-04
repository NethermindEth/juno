package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type blockIDKind string

const (
	blockIDNumber blockIDKind = "number"
	blockIDHash   blockIDKind = "hash"
	blockIDLatest blockIDKind = "latest"
)

func (k *blockIDKind) String() string { return string(*k) }

func (k *blockIDKind) Set(value string) error {
	switch kind := blockIDKind(value); kind {
	case blockIDNumber, blockIDHash, blockIDLatest:
		*k = kind
		return nil
	default:
		return fmt.Errorf("must be number, hash or latest (got %q)", value)
	}
}

func (k *blockIDKind) Type() string { return "string" }

func (k *blockIDKind) bind(cmd *cobra.Command, _ *rpcClient) {
	*k = blockIDNumber
	cmd.Flags().Var(k, "block-id", "Block id encoding: number, hash or latest.")
}

type blockIDArgs struct {
	blockRangeFlags
	BlockIDKind blockIDKind `json:"blockIdKind"`
}

func (a *blockIDArgs) bind(cmd *cobra.Command, client *rpcClient) {
	a.BlockIDKind.bind(cmd, client)
	a.blockRangeFlags.bind(cmd, client)
}

type blockIDWithProofFactsArgs struct {
	blockIDArgs
	ResponseFlags txnFlags `json:"responseFlags"`
}

func (a *blockIDWithProofFactsArgs) bind(cmd *cobra.Command, client *rpcClient) {
	a.ResponseFlags.bind(cmd, client)
	a.blockIDArgs.bind(cmd, client)
}

func resolveBlockID(
	ctx context.Context,
	client *rpcClient,
	kind blockIDKind,
	blockNumber uint64,
) (blockID, error) {
	switch kind {
	case blockIDHash:
		block, err := client.blockWithTxHashes(ctx, blockNumber)
		if err != nil {
			return nil, fmt.Errorf("resolve hash for block %d: %w", blockNumber, err)
		}
		return blockHashID{BlockHash: block.BlockHash, number: blockNumber}, nil
	case blockIDLatest:
		return latestBlockID{number: blockNumber}, nil
	case blockIDNumber:
		return blockNumberID{blockNumber}, nil
	default:
		return nil, fmt.Errorf("unknown block id kind %q", kind)
	}
}

func blockIDSampler(input samplerInput[blockIDArgs]) (blockIDParams, error) {
	id, err := resolveBlockID(
		input.ctx, input.client, input.args.BlockIDKind, input.args.sampleBlockNumber(input.rng),
	)
	if err != nil {
		return blockIDParams{}, err
	}
	return blockIDParams{BlockID: id}, nil
}

func blockIDWithProofFactsSampler(
	input samplerInput[blockIDWithProofFactsArgs],
) (blockIDParams, error) {
	params, err := blockIDSampler(input.rebindArgs(&input.args.blockIDArgs))
	if err != nil {
		return blockIDParams{}, err
	}
	params.ResponseFlags = input.args.ResponseFlags
	return params, nil
}

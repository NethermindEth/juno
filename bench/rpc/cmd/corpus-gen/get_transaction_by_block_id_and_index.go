package main

import (
	"errors"

	"github.com/spf13/cobra"
)

type txByBlockIDAndIndexArgs struct {
	blockIDWithProofFactsArgs
}

func (a *txByBlockIDAndIndexArgs) bind(cmd *cobra.Command, client *rpcClient) {
	chainPreRunE(cmd, func() error {
		if a.BlockIDKind == blockIDLatest {
			return errors.New("--block-id latest is not supported")
		}
		return nil
	})
	a.blockIDWithProofFactsArgs.bind(cmd, client)
}

func txByBlockIDAndIndexSampler(
	input samplerInput[txByBlockIDAndIndexArgs],
) (txByBlockIDAndIndexParams, error) {
	blockNumber := input.args.sampleBlockNumber(input.rng)
	count, err := input.client.txCountInBlock(input.ctx, blockNumber)
	if err != nil {
		return txByBlockIDAndIndexParams{}, err
	}
	if count == 0 {
		return txByBlockIDAndIndexParams{}, errResample
	}
	id, err := resolveBlockID(input.ctx, input.client, input.args.BlockIDKind, blockNumber)
	if err != nil {
		return txByBlockIDAndIndexParams{}, err
	}
	return txByBlockIDAndIndexParams{
		BlockID:       id,
		Index:         input.rng.Uint64N(count),
		ResponseFlags: input.args.ResponseFlags,
	}, nil
}

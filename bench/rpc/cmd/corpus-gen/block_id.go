package main

import (
	"context"
	"math/rand/v2"

	"github.com/spf13/cobra"
)

const (
	methodGetBlockWithTxs      = "starknet_getBlockWithTxs"
	methodGetBlockWithTxHashes = "starknet_getBlockWithTxHashes"
	methodGetBlockWithReceipts = "starknet_getBlockWithReceipts"
)

type blockNumberID struct {
	BlockNumber uint64 `json:"block_number"`
}

type blockIDParams struct {
	BlockID blockNumberID `json:"block_id"`
}

func newGetBlockWithTxsCmd(cfg *rootConfig) *cobra.Command {
	return newBlockIDCmd(cfg, "getBlockWithTxs",
		"Sample random block numbers and emit starknet_getBlockWithTxs requests.",
		methodGetBlockWithTxs)
}

func newGetBlockWithTxHashesCmd(cfg *rootConfig) *cobra.Command {
	return newBlockIDCmd(cfg, "getBlockWithTxHashes",
		"Sample random block numbers and emit starknet_getBlockWithTxHashes requests.",
		methodGetBlockWithTxHashes)
}

func newGetBlockWithReceiptsCmd(cfg *rootConfig) *cobra.Command {
	return newBlockIDCmd(cfg, "getBlockWithReceipts",
		"Sample random block numbers and emit starknet_getBlockWithReceipts requests.",
		methodGetBlockWithReceipts)
}

func newBlockIDCmd(cfg *rootConfig, use, short, method string) *cobra.Command {
	var blockStart, blockEnd uint64

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateBlockRange(blockStart, blockEnd); err != nil {
				return err
			}
			meta := blockRange{Start: blockStart, End: blockEnd}
			gen := func(_ context.Context, _ *rpcClient, rng *rand.Rand) (any, error) {
				return sampleBlockID(rng, blockStart, blockEnd), nil
			}
			return runCorpus(cmd, cfg, method, meta, gen)
		},
	}

	addBlockRangeFlags(cmd, &blockStart, &blockEnd)
	return cmd
}

func sampleBlockID(rng *rand.Rand, start, end uint64) blockIDParams {
	return blockIDParams{BlockID: blockNumberID{BlockNumber: start + rng.Uint64N(end-start)}}
}

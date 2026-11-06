package main

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"

	"github.com/spf13/cobra"
)

const (
	blockStartFlag = "block-start"
	blockEndFlag   = "block-end"
	blocksFlag     = "blocks"
)

// blockRangeFlags selects the blocks to sample: a uniform draw from Start..End,
// or, when Blocks is set, Blocks in order. Start and End then hold the list's
// bounds, so the sampling meta (and the seed) does not depend on the chain head.
type blockRangeFlags struct {
	Start  uint64   `json:"start"`
	End    uint64   `json:"end"`
	Blocks []uint64 `json:"blocks,omitempty"`
}

// sampleBlockNumber returns the block for the index-th request.
func (f blockRangeFlags) sampleBlockNumber(rng *rand.Rand, index int) uint64 {
	if len(f.Blocks) > 0 {
		return f.Blocks[index%len(f.Blocks)]
	}
	return uniformRange(rng, f.Start, f.End)
}

func (f *blockRangeFlags) bind(cmd *cobra.Command, client *rpcClient) {
	cmd.Flags().Uint64Var(
		&f.Start,
		blockStartFlag,
		0,
		"Start block number to sample from (inclusive).",
	)
	cmd.Flags().Uint64Var(
		&f.End,
		blockEndFlag,
		0,
		"End block number to sample from (inclusive). Defaults to the source node's latest block.",
	)
	var blocks []uint
	cmd.Flags().UintSliceVar(
		&blocks,
		blocksFlag,
		nil,
		"Sample these blocks in order, one per request, instead of a range. "+
			"--count defaults to one entry per block.",
	)
	cmd.MarkFlagsMutuallyExclusive(blocksFlag, blockStartFlag)
	cmd.MarkFlagsMutuallyExclusive(blocksFlag, blockEndFlag)
	chainPreRunE(cmd, func() error {
		latest, err := client.blockNumber(cmd.Context())
		if err != nil {
			return fmt.Errorf("fetch latest block number: %w", err)
		}
		if len(blocks) > 0 {
			f.Blocks = make([]uint64, len(blocks))
			for i, b := range blocks {
				f.Blocks[i] = uint64(b)
			}
			return f.useBlocks(cmd, latest)
		}
		if !cmd.Flags().Changed(blockEndFlag) {
			f.End = latest
		}
		if f.End > latest {
			return fmt.Errorf("--block-end (%d) must be <= the latest block (%d)", f.End, latest)
		}
		if f.Start > f.End {
			return fmt.Errorf("--block-start (%d) must be <= --block-end (%d)", f.Start, f.End)
		}
		return nil
	})
}

func (f *blockRangeFlags) useBlocks(cmd *cobra.Command, latest uint64) error {
	f.Start, f.End = slices.Min(f.Blocks), slices.Max(f.Blocks)
	if f.End > latest {
		return fmt.Errorf("--blocks has block %d, after the latest block (%d)", f.End, latest)
	}
	if cmd.Flags().Changed("count") {
		return nil
	}
	batch, err := cmd.Flags().GetInt("batch")
	if err != nil {
		return err
	}
	batchSize := max(1, batch)
	count := (len(f.Blocks) + batchSize - 1) / batchSize
	return cmd.Flags().Set("count", strconv.Itoa(count))
}

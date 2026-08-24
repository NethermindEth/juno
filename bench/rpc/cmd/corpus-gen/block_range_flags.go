package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/spf13/cobra"
)

const blockEndFlag = "block-end"

type blockRangeFlags struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
}

func (f blockRangeFlags) sampleBlockNumber(rng *rand.Rand) uint64 {
	return f.Start + rng.Uint64N(f.End-f.Start+1)
}

func (f *blockRangeFlags) bind(cmd *cobra.Command, client *rpcClient) {
	cmd.Flags().Uint64Var(
		&f.Start,
		"block-start",
		0,
		"Start block number to sample from (inclusive).",
	)
	cmd.Flags().Uint64Var(
		&f.End,
		blockEndFlag,
		0,
		"End block number to sample from (inclusive). Defaults to the source node's latest block.",
	)
	chainPreRunE(cmd, func() error {
		latest, err := client.blockNumber(cmd.Context())
		if err != nil {
			return fmt.Errorf("fetch latest block number: %w", err)
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

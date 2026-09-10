package main

import (
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/spf13/cobra"
)

const (
	blockStartFlag = "block-start"
	blockEndFlag   = "block-end"
)

// defaultExecutionWindow keeps the default range of the execution methods near
// the head, where the state they replay against is still available.
const defaultExecutionWindow = 10000

type blockRangeFlags struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`

	// headroom holds blocks back from the top of the range; a sampler that
	// reads block N+1 reserves one.
	headroom uint64
}

func (f blockRangeFlags) sampleBlockNumber(rng *rand.Rand) uint64 {
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
	chainPreRunE(cmd, func() error {
		latest, err := client.blockNumber(cmd.Context())
		if err != nil {
			return fmt.Errorf("fetch latest block number: %w", err)
		}
		maxEnd := latest - min(latest, f.headroom)
		if !cmd.Flags().Changed(blockEndFlag) {
			f.End = maxEnd
		}
		if f.End > maxEnd {
			if f.headroom == 0 {
				return fmt.Errorf("--block-end (%d) must be <= the latest block (%d)", f.End, latest)
			}
			return fmt.Errorf(
				"--block-end (%d) must be <= %d, so that the next block exists", f.End, maxEnd,
			)
		}
		if f.Start > f.End {
			return fmt.Errorf("--block-start (%d) must be <= --block-end (%d)", f.Start, f.End)
		}
		return nil
	})
}

// bindExecutionRange binds the block range of an execution method: it reserves
// the head block, since the transactions come from block N+1, and starts near
// the head unless the caller sets --block-start.
func bindExecutionRange(cmd *cobra.Command, client *rpcClient, args *blockIDArgs) {
	args.headroom = 1
	args.bind(cmd, client)
	cmd.Flags().Lookup(blockStartFlag).Usage = fmt.Sprintf(
		"Start block number to sample from (inclusive). Defaults to %d blocks below --block-end.",
		defaultExecutionWindow,
	)
	chainPreRunE(cmd, func() error {
		if args.BlockIDKind == blockIDLatest {
			return errors.New("--block-id latest is not supported")
		}
		if !cmd.Flags().Changed(blockStartFlag) {
			args.Start = args.End - min(args.End, defaultExecutionWindow)
		}
		return nil
	})
}

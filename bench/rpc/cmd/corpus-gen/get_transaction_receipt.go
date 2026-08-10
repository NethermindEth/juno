package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/spf13/cobra"
)

const methodGetTransactionReceipt = "starknet_getTransactionReceipt"

// newGetTxReceiptCmd reuses getTransactionByHash's sampler: both are keyed by a
// transaction hash.
func newGetTxReceiptCmd(cfg *rootConfig) *cobra.Command {
	var blockStart, blockEnd uint64

	cmd := &cobra.Command{
		Use:   "getTransactionReceipt",
		Short: "Sample random transaction hashes and emit starknet_getTransactionReceipt requests.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if blockEnd <= blockStart {
				return fmt.Errorf("--block-end (%d) must be > --block-start (%d)", blockEnd, blockStart)
			}
			meta := blockRange{Start: blockStart, End: blockEnd}
			makeGen := func(client *rpcClient, rng *rand.Rand) func() (any, error) {
				return func() (any, error) {
					return sampleTxHash(cmd.Context(), client, rng, blockStart, blockEnd)
				}
			}
			return runCorpus(cmd, cfg, methodGetTransactionReceipt, meta, makeGen)
		},
	}

	cmd.Flags().Uint64Var(&blockStart, "block-start", 0,
		"Start block number to sample from (inclusive).")
	cmd.Flags().Uint64Var(&blockEnd, "block-end", 0,
		"End block number to sample from (exclusive).")
	_ = cmd.MarkFlagRequired("block-start")
	_ = cmd.MarkFlagRequired("block-end")
	return cmd
}

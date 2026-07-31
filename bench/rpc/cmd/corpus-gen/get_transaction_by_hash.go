package main

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/spf13/cobra"
)

const (
	methodGetTransactionByHash = "starknet_getTransactionByHash"
	maxResampleAttempts        = 10000
)

type getTxByHashParams struct {
	TransactionHash string `json:"transaction_hash"`
}

func newGetTxByHashCmd(cfg *rootConfig) *cobra.Command {
	var blockStart, blockEnd uint64

	cmd := &cobra.Command{
		Use:   "getTransactionByHash",
		Short: "Sample random transaction hashes and emit starknet_getTransactionByHash requests.",
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
			return runCorpus(cmd, cfg, methodGetTransactionByHash, meta, makeGen)
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

func sampleTxHash(
	ctx context.Context,
	client *rpcClient,
	rng *rand.Rand,
	start, end uint64,
) (getTxByHashParams, error) {
	span := end - start
	for range maxResampleAttempts {
		blockNumber := start + rng.Uint64N(span)
		hashes, err := txHashesInBlock(ctx, client, blockNumber)
		if err != nil {
			return getTxByHashParams{}, fmt.Errorf("get block %d: %w", blockNumber, err)
		}
		if len(hashes) == 0 {
			continue
		}
		return getTxByHashParams{TransactionHash: hashes[rng.Uint64N(uint64(len(hashes)))]}, nil
	}
	return getTxByHashParams{}, fmt.Errorf(
		"no non-empty block found in [%d, %d) after %d attempts", start, end, maxResampleAttempts)
}

func txHashesInBlock(ctx context.Context, client *rpcClient, blockNumber uint64) ([]string, error) {
	type block struct {
		Transactions []string `json:"transactions"`
	}
	params := map[string]any{"block_id": map[string]any{"block_number": blockNumber}}
	result, err := rpcCall[block](ctx, client, "starknet_getBlockWithTxHashes", params)
	if err != nil {
		return nil, err
	}
	return result.Transactions, nil
}

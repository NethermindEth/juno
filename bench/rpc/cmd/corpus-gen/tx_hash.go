package main

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/spf13/cobra"
)

const (
	methodGetTransactionByHash  = "starknet_getTransactionByHash"
	methodGetTransactionReceipt = "starknet_getTransactionReceipt"
	maxResampleAttempts         = 10000
)

type txHashParams struct {
	TransactionHash string `json:"transaction_hash"`
}

func newGetTxByHashCmd(cfg *rootConfig) *cobra.Command {
	return newTxHashCmd(cfg, "getTransactionByHash",
		"Sample random transaction hashes and emit starknet_getTransactionByHash requests.",
		methodGetTransactionByHash)
}

func newGetTxReceiptCmd(cfg *rootConfig) *cobra.Command {
	return newTxHashCmd(cfg, "getTransactionReceipt",
		"Sample random transaction hashes and emit starknet_getTransactionReceipt requests.",
		methodGetTransactionReceipt)
}

func newTxHashCmd(cfg *rootConfig, use, short, method string) *cobra.Command {
	var blockStart, blockEnd uint64

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateBlockRange(blockStart, blockEnd); err != nil {
				return err
			}
			meta := blockRange{Start: blockStart, End: blockEnd}
			gen := func(ctx context.Context, client *rpcClient, rng *rand.Rand) (any, error) {
				return sampleTxHash(ctx, client, rng, blockStart, blockEnd)
			}
			return runCorpus(cmd, cfg, method, meta, gen)
		},
	}

	addBlockRangeFlags(cmd, &blockStart, &blockEnd)
	return cmd
}

func sampleTxHash(
	ctx context.Context,
	client *rpcClient,
	rng *rand.Rand,
	start, end uint64,
) (txHashParams, error) {
	span := end - start
	for range maxResampleAttempts {
		blockNumber := start + rng.Uint64N(span)
		hashes, err := txHashesInBlock(ctx, client, blockNumber)
		if err != nil {
			return txHashParams{}, fmt.Errorf("get block %d: %w", blockNumber, err)
		}
		if len(hashes) == 0 {
			continue
		}
		return txHashParams{TransactionHash: hashes[rng.Uint64N(uint64(len(hashes)))]}, nil
	}
	return txHashParams{}, fmt.Errorf(
		"no non-empty block found in [%d, %d) after %d attempts", start, end, maxResampleAttempts)
}

func txHashesInBlock(ctx context.Context, client *rpcClient, blockNumber uint64) ([]string, error) {
	type block struct {
		Transactions []string `json:"transactions"`
	}
	params := blockIDParams{BlockID: blockNumberID{BlockNumber: blockNumber}}
	result, err := rpcCall[block](ctx, client, methodGetBlockWithTxHashes, params)
	if err != nil {
		return nil, err
	}
	return result.Transactions, nil
}

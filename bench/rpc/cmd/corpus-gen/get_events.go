package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

const (
	defaultMaxWindow   = 10
	defaultChunkSize   = 100
	defaultAddressProb = 0.5
	maxEmitterAttempts = 10
)

type eventsArgs struct {
	blockIDArgs
	MaxWindow   uint64  `json:"maxWindow"`
	ChunkSize   int     `json:"chunkSize"`
	AddressProb float64 `json:"addressProb"`
}

func (a *eventsArgs) bind(cmd *cobra.Command, client *rpcClient) {
	cmd.Flags().Uint64Var(
		&a.MaxWindow,
		"max-window",
		defaultMaxWindow,
		"Maximum number of blocks per event filter.",
	)
	cmd.Flags().IntVar(
		&a.ChunkSize,
		"chunk-size",
		defaultChunkSize,
		"chunk_size value for each request.",
	)
	cmd.Flags().Float64Var(
		&a.AddressProb,
		"address-prob",
		defaultAddressProb,
		"Probability that a request filters by an emitting contract address.",
	)
	chainPreRunE(cmd, func() error {
		if a.BlockIDKind == blockIDLatest {
			return errors.New("--block-id latest is not supported")
		}
		if a.MaxWindow < 1 {
			return fmt.Errorf("--max-window must be >= 1 (got %d)", a.MaxWindow)
		}
		if a.ChunkSize < 1 {
			return fmt.Errorf("--chunk-size must be >= 1 (got %d)", a.ChunkSize)
		}
		if a.AddressProb < 0 || a.AddressProb > 1 {
			return fmt.Errorf("--address-prob must be in [0, 1] (got %g)", a.AddressProb)
		}
		return nil
	})
	a.blockIDArgs.bind(cmd, client)
}

func eventsSampler(input samplerInput[eventsArgs]) (eventsParams, error) {
	from := input.args.sampleBlockNumber(input.rng)
	to := from + input.rng.Uint64N(min(input.args.MaxWindow, input.args.End-from+1))
	fromID, err := resolveBlockID(input.ctx, input.client, input.args.BlockIDKind, from)
	if err != nil {
		return eventsParams{}, err
	}
	toID, err := resolveBlockID(input.ctx, input.client, input.args.BlockIDKind, to)
	if err != nil {
		return eventsParams{}, err
	}
	filter := eventFilter{
		FromBlock: fromID,
		ToBlock:   toID,
		ChunkSize: input.args.ChunkSize,
	}
	if input.rng.Float64() < input.args.AddressProb {
		// Pick an emitter from [from, to], weighted by event count; errResample
		// when none found (an empty address would silently become an unfiltered query).
		span := to - from + 1
		var emitters []string
		for range min(span, maxEmitterAttempts) {
			block, err := input.client.blockWithReceipts(input.ctx, from+input.rng.Uint64N(span))
			if err != nil {
				return eventsParams{}, err
			}
			if emitters = eventEmitters(block); len(emitters) > 0 {
				break
			}
		}
		address, err := pickRandom(input.rng, emitters)
		if err != nil {
			return eventsParams{}, err
		}
		filter.Address = address
	}
	return eventsParams{Filter: filter}, nil
}

func eventEmitters(block receiptsBlock) []string {
	var emitters []string
	for _, tx := range block.Transactions {
		for _, event := range tx.Receipt.Events {
			emitters = append(emitters, event.FromAddress)
		}
	}
	return emitters
}

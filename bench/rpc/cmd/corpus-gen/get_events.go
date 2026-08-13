package main

import (
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
	blockRangeFlags
	MaxWindow   uint64  `json:"maxWindow"`
	ChunkSize   int     `json:"chunkSize"`
	AddressProb float64 `json:"addressProb"`
}

func eventsExtraArgs(cmd *cobra.Command, args *eventsArgs) {
	addBlockRangeFlags(cmd, &args.blockRangeFlags)
	cmd.Flags().Uint64Var(
		&args.MaxWindow,
		"max-window",
		defaultMaxWindow,
		"Maximum number of blocks per event filter.",
	)
	cmd.Flags().IntVar(
		&args.ChunkSize,
		"chunk-size",
		defaultChunkSize,
		"chunk_size value for each request.",
	)
	cmd.Flags().Float64Var(
		&args.AddressProb,
		"address-prob",
		defaultAddressProb,
		"Probability that a request filters by an emitting contract address.",
	)
	chainPreRunE(cmd, func() error {
		if args.MaxWindow < 1 {
			return fmt.Errorf("--max-window must be >= 1 (got %d)", args.MaxWindow)
		}
		if args.ChunkSize < 1 {
			return fmt.Errorf("--chunk-size must be >= 1 (got %d)", args.ChunkSize)
		}
		if args.AddressProb < 0 || args.AddressProb > 1 {
			return fmt.Errorf("--address-prob must be in [0, 1] (got %g)", args.AddressProb)
		}
		return nil
	})
}

func eventsSampler(input samplerInput[eventsArgs]) (any, error) {
	from := input.args.sampleBlockNumber(input.rng)
	to := from + input.rng.Uint64N(min(input.args.MaxWindow, input.args.End-from+1))
	filter := eventFilter{
		FromBlock: blockNumberID{from},
		ToBlock:   blockNumberID{to},
		ChunkSize: input.args.ChunkSize,
	}
	if input.rng.Float64() < input.args.AddressProb {
		address, err := sampleEmitter(input, from, to)
		if err != nil {
			return nil, err
		}
		filter.Address = address
	}
	return eventsParams{Filter: filter}, nil
}

// sampleEmitter returns the address of a contract that emitted an event in
// [from, to], weighted by event count, or errResample when no sampled block
// has events (an empty address would silently become an unfiltered query).
func sampleEmitter(input samplerInput[eventsArgs], from, to uint64) (string, error) {
	span := to - from + 1
	for range min(span, maxEmitterAttempts) {
		block, err := input.client.blockWithReceipts(input.ctx, from+input.rng.Uint64N(span))
		if err != nil {
			return "", err
		}
		emitters := eventEmitters(block)
		if len(emitters) > 0 {
			return emitters[input.rng.Uint64N(uint64(len(emitters)))], nil
		}
	}
	return "", errResample
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

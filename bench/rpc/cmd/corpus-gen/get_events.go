package main

import (
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/spf13/cobra"
)

const (
	defaultWindow    = 100
	defaultChunkSize = 1000
)

type eventsArgs struct {
	blockIDArgs
	MinWindow    uint64 `json:"minWindow"`
	MaxWindow    uint64 `json:"maxWindow"`
	MinChunkSize uint64 `json:"minChunkSize"`
	MaxChunkSize uint64 `json:"maxChunkSize"`
	Addresses    int    `json:"addresses"`
	Keys         []int  `json:"keys,omitempty"`
}

func (a *eventsArgs) fullRange() bool {
	return a.MaxWindow == 0
}

func (a *eventsArgs) bind(cmd *cobra.Command, client *rpcClient) {
	var window, chunkSize []uint
	cmd.Flags().UintSliceVar(
		&window,
		"window",
		[]uint{defaultWindow},
		"Blocks per event filter (to_block - from_block + 1): one value or min,max. "+
			"A single 0 omits from_block and to_block (whole chain).",
	)
	cmd.Flags().UintSliceVar(
		&chunkSize,
		"chunk-size",
		[]uint{defaultChunkSize},
		"chunk_size value for each request: one value or min,max.",
	)
	cmd.Flags().IntVar(
		&a.Addresses,
		"addresses",
		0,
		"Number of emitter addresses in the filter; 0 omits the address filter.",
	)
	cmd.Flags().IntSliceVar(
		&a.Keys,
		"keys",
		nil,
		"Key counts per position (e.g. 1,0,2); 0 is a wildcard position. Omit for no keys filter.",
	)
	a.blockIDArgs.bind(cmd, client)
	chainPreRunE(cmd, func() error {
		if a.BlockIDKind == blockIDLatest {
			return errors.New("--block-id latest is not supported")
		}
		if len(window) == 1 && window[0] == 0 {
			if cmd.Flags().Changed("block-id") {
				return errors.New("--block-id has no effect with --window 0")
			}
		} else {
			var err error
			if a.MinWindow, a.MaxWindow, err = parseBounds("window", window); err != nil {
				return err
			}
			if rangeSize := a.End - a.Start + 1; a.MaxWindow > rangeSize {
				return fmt.Errorf(
					"--window max (%d) must be <= the block range size (%d)", a.MaxWindow, rangeSize,
				)
			}
		}
		var err error
		if a.MinChunkSize, a.MaxChunkSize, err = parseBounds("chunk-size", chunkSize); err != nil {
			return err
		}
		if a.Addresses < 0 {
			return fmt.Errorf("--addresses must be >= 0 (got %d)", a.Addresses)
		}
		for position, count := range a.Keys {
			if count < 0 {
				return fmt.Errorf(
					"--keys counts must be >= 0 (got %d at position %d)", count, position,
				)
			}
		}
		return nil
	})
}

func parseBounds(flag string, values []uint) (lo, hi uint64, err error) {
	if len(values) < 1 || len(values) > 2 {
		return 0, 0, fmt.Errorf("--%s expects 1 or 2 values (got %d)", flag, len(values))
	}
	lo, hi = uint64(values[0]), uint64(values[len(values)-1])
	if lo < 1 {
		return 0, 0, fmt.Errorf("--%s values must be >= 1 (got %d)", flag, lo)
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("--%s min (%d) must be <= max (%d)", flag, lo, hi)
	}
	return lo, hi, nil
}

func eventsSampler(input samplerInput[eventsArgs]) (eventsParams, error) {
	args := input.args
	rng := input.rng

	anchor, base, events, err := sampleAnchor(input)
	if err != nil {
		return eventsParams{}, err
	}

	fromID, toID, err := sampleBlockIDs(input, anchor)
	if err != nil {
		return eventsParams{}, err
	}

	filter := eventFilter{
		FromBlock: fromID,
		ToBlock:   toID,
		Address:   sampleAddresses(rng, args.Addresses, base, events),
		Keys:      sampleEventKeys(rng, args.Keys, base, events),
		ChunkSize: uniformRange(rng, args.MinChunkSize, args.MaxChunkSize),
	}
	return eventsParams{Filter: filter}, nil
}

func sampleAnchor(input samplerInput[eventsArgs]) (uint64, receiptEvent, []receiptEvent, error) {
	number := input.args.sampleBlockNumber(input.rng)
	if input.args.Addresses == 0 && len(input.args.Keys) == 0 {
		return number, receiptEvent{}, nil, nil
	}
	block, err := input.client.blockWithReceipts(input.ctx, number)
	if err != nil {
		return 0, receiptEvent{}, nil, err
	}
	events, eligible := blockEvents(block, len(input.args.Keys))
	base, err := pickRandom(input.rng, eligible)
	if err != nil {
		return 0, receiptEvent{}, nil, err
	}
	return number, base, events, nil
}

func sampleBlockIDs(
	input samplerInput[eventsArgs],
	anchor uint64,
) (fromID, toID blockID, err error) {
	if input.args.fullRange() {
		return nil, nil, nil
	}
	from, to := sampleWindow(input.rng, input.args, anchor)
	fromID, err = resolveBlockID(input.ctx, input.client, input.args.BlockIDKind, from)
	if err != nil {
		return nil, nil, err
	}
	toID, err = resolveBlockID(input.ctx, input.client, input.args.BlockIDKind, to)
	return fromID, toID, err
}

func blockEvents(block receiptsBlock, minKeys int) (events, eligible []receiptEvent) {
	for _, tx := range block.Transactions {
		for _, event := range tx.Receipt.Events {
			events = append(events, event)
			if len(event.Keys) >= minKeys {
				eligible = append(eligible, event)
			}
		}
	}
	return events, eligible
}

func sampleAddresses(
	rng *rand.Rand,
	count int,
	base receiptEvent,
	events []receiptEvent,
) addressList {
	if count == 0 {
		return nil
	}
	set := newOrderedSet()
	set.add(base.FromAddress)
	for _, event := range events {
		set.add(event.FromAddress)
	}
	return addressList(fillDistinct(rng, count, base.FromAddress, set.items[1:]))
}

func sampleEventKeys(
	rng *rand.Rand,
	keyCounts []int,
	base receiptEvent,
	events []receiptEvent,
) [][]string {
	if len(keyCounts) == 0 {
		return nil
	}
	keys := make([][]string, len(keyCounts))
	for position, count := range keyCounts {
		if count == 0 {
			keys[position] = []string{}
			continue
		}
		baseKey := base.Keys[position]
		set := newOrderedSet()
		set.add(baseKey)
		for _, event := range events {
			if position < len(event.Keys) {
				set.add(event.Keys[position])
			}
		}
		keys[position] = fillDistinct(rng, count, baseKey, set.items[1:])
	}
	return keys
}

func fillDistinct(rng *rand.Rand, n int, base string, candidates []string) []string {
	picked := make([]string, 1, n)
	picked[0] = base
	for _, i := range rng.Perm(len(candidates)) {
		if len(picked) == n {
			break
		}
		picked = append(picked, candidates[i])
	}
	for len(picked) < n {
		picked = append(picked, randomFelt(rng))
	}
	swap := rng.IntN(len(picked))
	picked[0], picked[swap] = picked[swap], picked[0]
	return picked
}

func randomFelt(rng *rand.Rand) string {
	return fmt.Sprintf("0x%x%016x", rng.Uint64()|1<<63, rng.Uint64())
}

func sampleWindow(rng *rand.Rand, args *eventsArgs, anchor uint64) (from, to uint64) {
	window := uniformRange(rng, args.MinWindow, args.MaxWindow)
	lo := args.Start
	if anchor+1 > window && anchor+1-window > lo {
		lo = anchor + 1 - window
	}
	hi := min(anchor, args.End-window+1)
	from = uniformRange(rng, lo, hi)
	return from, from + window - 1
}

type orderedSet struct {
	seen  map[string]bool
	items []string
}

func newOrderedSet() *orderedSet {
	return &orderedSet{seen: map[string]bool{}}
}

func (s *orderedSet) add(item string) {
	if !s.seen[item] {
		s.seen[item] = true
		s.items = append(s.items, item)
	}
}

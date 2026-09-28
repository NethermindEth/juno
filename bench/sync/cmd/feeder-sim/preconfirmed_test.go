package main

import (
	"errors"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/require"
)

func TestPreConfirmedReply(t *testing.T) {
	store := preConfirmedStore(t)
	rounds := decodedRounds(t)
	middle := fixtureFrom + 1
	// preConfirmedConfig reveals the top block in 3 stages, so its interval has 4 phases.
	phase := time.Hour / 4

	// With the tip at fixtureFrom the window is [56377, 56379] and 56379 (40 transactions)
	// fills: 0, 13, 26 then 40 revealed. 56377 has 45 transactions and 56378 has 51.
	tests := []struct {
		name       string
		tip        uint64
		elapsed    time.Duration
		block      uint64
		latest     bool
		resume     bool   // send the block's own identifier
		identifier string // sent when not resuming; empty sends the blank identifier
		known      uint64
		want       replyKind
		from, to   uint64
	}{
		{name: "tip block", tip: fixtureFrom, block: fixtureFrom, want: wantFull, to: 45},
		{name: "complete block above the tip", tip: fixtureFrom, block: middle, want: wantFull, to: 51},
		{
			name: "filling block at the first phase", tip: fixtureFrom, elapsed: phase / 2,
			block: fixtureTo, want: wantFull,
		},
		{
			name: "filling block at the second phase", tip: fixtureFrom, elapsed: phase + phase/2,
			block: fixtureTo, want: wantFull, to: 13,
		},
		{
			name: "filling block at the last phase", tip: fixtureFrom, elapsed: 3*phase + phase/2,
			block: fixtureTo, want: wantFull, to: 40,
		},
		{
			name: "filling block past the interval", tip: fixtureFrom, elapsed: 2 * time.Hour,
			block: fixtureTo, want: wantFull, to: 40,
		},
		{
			name: "latest at the first phase", tip: fixtureFrom, elapsed: phase / 2,
			block: fixtureTo, latest: true, want: wantFull,
		},
		{
			name: "latest at the third phase", tip: fixtureFrom, elapsed: 2*phase + phase/2,
			block: fixtureTo, latest: true, want: wantFull, to: 26,
		},
		{
			name: "resume unchanged", tip: fixtureFrom, block: fixtureFrom, resume: true, known: 45,
			want: wantNoChange,
		},
		{
			name: "resume ahead of the reveal", tip: fixtureFrom, elapsed: phase + phase/2,
			block: fixtureTo, resume: true, known: 20, want: wantNoChange,
		},
		{
			name: "resume before anything is revealed", tip: fixtureFrom, elapsed: phase / 2,
			block: fixtureTo, resume: true, want: wantNoChange,
		},
		{
			name: "resume filling block", tip: fixtureFrom, elapsed: 2*phase + phase/2,
			block: fixtureTo, resume: true, known: 13, want: wantDelta, from: 13, to: 26,
		},
		{
			name: "resume complete block", tip: fixtureFrom, block: middle, resume: true, known: 20,
			want: wantDelta, from: 20, to: 51,
		},
		{
			name: "resume complete block from zero", tip: fixtureFrom, block: middle, resume: true,
			want: wantFull, to: 51,
		},
		{
			name: "resume filling block from zero", tip: fixtureFrom, elapsed: phase + phase/2,
			block: fixtureTo, resume: true, want: wantFull, to: 13,
		},
		{
			name: "resume latest", tip: fixtureFrom, elapsed: 3*phase + phase/2,
			block: fixtureTo, latest: true, resume: true, known: 13, want: wantDelta, from: 13, to: 40,
		},
		{
			name: "resume latest unchanged", tip: fixtureFrom, elapsed: phase + phase/2,
			block: fixtureTo, latest: true, resume: true, known: 13, want: wantNoChange,
		},
		{
			name: "other identifier resets the known count", tip: fixtureFrom, block: fixtureFrom,
			identifier: "0xdead", known: 10, want: wantFull, to: 45,
		},
		{
			name: "latest with other identifier", tip: fixtureFrom, elapsed: phase + phase/2,
			block: fixtureTo, latest: true, identifier: "0xdead", known: 5, want: wantFull, to: 13,
		},
		{
			name: "top block clamped to to is complete", tip: middle, elapsed: phase / 2,
			block: fixtureTo, latest: true, want: wantFull, to: 40,
		},
		{name: "kept below the tip", tip: middle, block: fixtureFrom, want: wantFull, to: 45},
		{name: "tip block at to", tip: fixtureTo, block: fixtureTo, want: wantFull, to: 40},
		{name: "below the window", tip: fixtureTo, block: fixtureFrom, want: wantNotFound},
		{name: "latest at to", tip: fixtureTo, latest: true, want: wantNotFound},
		{name: "above to", tip: fixtureFrom, block: fixtureTo + 1, want: wantNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				simulator := newPipeSimulator(t, store, preConfirmedConfig(test.tip))
				time.Sleep(test.elapsed)

				identifier := test.identifier
				if test.resume {
					identifier = rounds[test.block].BlockIdentifier
				}

				var (
					update starknet.PreConfirmedUpdate
					number uint64
					err    error
				)
				if test.latest {
					update, number, err = simulator.client.PreConfirmedBlockLatest(
						t.Context(), identifier, test.known,
					)
				} else {
					update, err = simulator.client.PreConfirmedBlockWithIdentifier(
						t.Context(), strconv.FormatUint(test.block, 10), identifier, test.known,
					)
				}

				if test.want == wantNotFound {
					require.ErrorIs(t, err, feeder.ErrPreConfirmedBlockNotFound)
					return
				}

				require.NoError(t, err)
				require.Equal(t, wantUpdate(rounds[test.block], test.want, test.from, test.to), update)
				if test.latest && test.want != wantNoChange {
					require.Equal(t, test.block, number)
				}
			})
		})
	}
}

// TestPreConfirmedDeltasRebuildBlock follows blockNumber=latest the way Juno's poller does and
// checks the full blocks and deltas it applies add up to each captured round.
func TestPreConfirmedDeltasRebuildBlock(t *testing.T) {
	store := preConfirmedStore(t)
	rounds := decodedRounds(t)
	middle := fixtureFrom + 1
	// Multiples of 300ms never land on a 4s tick, so a poll never races a tip advance.
	const poll = 300 * time.Millisecond

	tests := []struct {
		name   string
		stages uint64
		want   map[uint64][]uint64 // transaction counts the follower holds, per block
	}{
		{"all at once", 0, map[uint64][]uint64{middle: {51}, fixtureTo: {40}}},
		{"one stage", 1, map[uint64][]uint64{middle: {0, 51}, fixtureTo: {0, 40}}},
		{"three stages", 3, map[uint64][]uint64{middle: {0, 17, 34, 51}, fixtureTo: {0, 13, 26, 40}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				config := preConfirmedConfig(fixtureFrom)
				config.lead = 1
				config.keep = 0
				config.stages = test.stages
				config.interval = 4 * time.Second
				simulator := newPipeSimulator(t, store, config)
				go simulator.clock.run(t.Context())
				go simulator.window.run(t.Context(), simulator.clock.advanced)

				seen := make(map[uint64][]uint64)
				followed := make(map[uint64]pending.PreConfirmed)
				var (
					current pending.PreConfirmed
					number  uint64
					known   uint64
				)
				// The tip reaches --to after two intervals, which empties the window.
				deadline := time.Now().Add(3 * config.interval)
				for {
					require.True(t, time.Now().Before(deadline), "latest outlived the tip reaching --to")
					update, latest, err := simulator.client.PreConfirmedBlockLatest(
						t.Context(), current.BlockIdentifier, known,
					)
					if errors.Is(err, feeder.ErrPreConfirmedBlockNotFound) {
						break
					}
					require.NoError(t, err)

					switch update := update.(type) {
					case starknet.PreConfirmedBlock:
						require.True(
							t, latest != number || known == 0,
							"a full block mid-round only answers a follower holding nothing",
						)
						number = latest
						current, err = sn2core.AdaptPreConfirmedBlock(&update, number)
					case starknet.PreConfirmedDeltaUpdate:
						current, err = sn2core.AdaptPreConfirmedWithDelta(&current, &update)
					}
					require.NoError(t, err)

					known = uint64(len(current.Block.Transactions))
					if counts := seen[number]; len(counts) == 0 || counts[len(counts)-1] != known {
						seen[number] = append(counts, known)
					}
					followed[number] = current
					time.Sleep(poll)
				}

				require.Equal(t, test.want, seen)
				for number, got := range followed {
					want, err := sn2core.AdaptPreConfirmedBlock(rounds[number], number)
					require.NoError(t, err)
					require.Equal(t, want, got, "block %d", number)
				}
			})
		})
	}
}

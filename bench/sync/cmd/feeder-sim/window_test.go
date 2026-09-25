package main

import (
	"bytes"
	"maps"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

func roundFixture(t *testing.T, number uint64, body string) fixture {
	t.Helper()
	return fixture{mustResource(t, preConfirmedBlock, blockKey{BlockNumber: number}), []byte(body)}
}

func TestNewWindowRejects(t *testing.T) {
	mismatched := `{"changed": true, "block_identifier": "0x1", "transactions": [{}]}`
	logger := log.NewNopZapLogger()
	withoutRounds, err := loadStore(t.Context(), fixtureDataset(t), fixtureConfig(), logger)
	require.NoError(t, err)

	tests := []struct {
		name    string
		store   *store
		wantErr string
	}{
		{"rounds not loaded", withoutRounds, "get_preconfirmed_block/56377.json.gz: not in dataset"},
		{
			"corrupt round below the top",
			preConfirmedStore(t, roundFixture(t, fixtureFrom+1, mismatched)),
			"get_preconfirmed_block/56378.json.gz: 1 transactions, 0 receipts, 0 state diffs",
		},
		{
			"corrupt top round",
			preConfirmedStore(t, roundFixture(t, fixtureTo, mismatched)),
			"get_preconfirmed_block/56379.json.gz: 1 transactions, 0 receipts, 0 state diffs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := preConfirmedConfig(fixtureFrom)
			start := advance{tip: fixtureFrom, interval: config.interval}
			_, err := newWindow(test.store, config, logger, start)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestWindowSpeculate(t *testing.T) {
	complete := preConfirmedStore(t)
	rounds := decodedRounds(t)
	tests := []struct {
		name      string
		store     *store
		tip       uint64
		wantOK    bool
		wantBlock uint64
	}{
		{"next block", complete, fixtureFrom, true, fixtureTo},
		{"past to", complete, fixtureFrom + 1, false, 0},
		{
			"corrupt round",
			preConfirmedStore(t, roundFixture(t, fixtureTo, `{"transactions": [{}]}`)),
			fixtureFrom, false, 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := preConfirmedConfig(test.tip)
			config.lead = 1
			start := advance{tip: test.tip, interval: config.interval}
			window, err := newWindow(test.store, config, log.NewNopZapLogger(), start)
			require.NoError(t, err)

			block, latest, ok := window.speculate()
			require.Equal(t, test.wantOK, ok)
			if !test.wantOK {
				require.Nil(t, block.round)
				require.Nil(t, latest)
				return
			}

			require.Equal(t, rounds[test.wantBlock].BlockIdentifier, block.round.BlockIdentifier)
			checkLatest(t, rounds[test.wantBlock], test.wantBlock, latest, config.stages)
		})
	}
}

func TestWindowRun(t *testing.T) {
	store := preConfirmedStore(t)
	rounds := decodedRounds(t)
	middle := fixtureFrom + 1
	whole := []uint64{fixtureFrom, middle, fixtureTo}

	tests := []struct {
		name       string
		lead, keep uint64
		want       [][]uint64 // blocks in the window at each tip, from fixtureFrom to fixtureTo
	}{
		{"lead 1 keep 0", 1, 0, [][]uint64{{fixtureFrom, middle}, {middle, fixtureTo}, {fixtureTo}}},
		{"lead 2 keep 1", 2, 1, [][]uint64{whole, whole, {middle, fixtureTo}}},
		{"window covers the range", 3, 5, [][]uint64{whole, whole, whole}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				config := preConfirmedConfig(fixtureFrom)
				config.lead = test.lead
				config.keep = test.keep
				config.interval = 2 * time.Second
				logger := log.NewNopZapLogger()
				clock := newClock(store.blocks, config, logger)
				window, err := newWindow(store, config, logger, clock.position())
				require.NoError(t, err)
				go clock.run(t.Context())
				go window.run(t.Context(), clock.advanced)

				previous := window.current.Load()
				// Sample mid-interval so a sample never races a tick.
				time.Sleep(config.interval / 2)
				for index, want := range test.want {
					current := window.current.Load()
					require.Equal(t, fixtureFrom+uint64(index), current.tip)
					require.ElementsMatch(t, want, slices.Collect(maps.Keys(current.blocks)))

					top := slices.Max(want)
					checkLatest(t, rounds[top], top, current.latest, config.stages)
					for number, block := range current.blocks {
						if kept, ok := previous.blocks[number]; ok {
							require.Same(t, kept.round, block.round, "block %d is prepared again", number)
						}
					}

					previous = current
					time.Sleep(config.interval)
				}
			})
		})
	}
}

// checkLatest checks latest holds, per phase, the top block's revealed transactions as Juno
// decodes a reply to blockNumber=latest.
func checkLatest(
	t *testing.T,
	round *starknet.PreConfirmedBlock,
	number uint64,
	latest [][]byte,
	stages uint64,
) {
	t.Helper()
	require.Len(t, latest, int(stages)+1)
	total := uint64(len(round.Transactions))
	for phase, body := range latest {
		plain, err := gunzip(body)
		require.NoError(t, err)
		envelope, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(plain))
		require.NoError(t, err)
		require.NoError(t, envelope.Validate())
		require.Equal(t, number, envelope.BlockNumber)
		shown := revealed(total, stages, uint64(phase))
		require.Equal(t, wantUpdate(round, wantFull, 0, shown), envelope.Update, "phase %d", phase)
	}
}

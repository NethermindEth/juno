package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSnapshotBounds(t *testing.T) {
	tests := []struct {
		name           string
		tip            uint64
		keep, lead     uint64
		wantLo, wantHi uint64
	}{
		{"tip at from", 10, 5, 3, 10, 13},
		{"keep reaches from", 15, 5, 3, 10, 18},
		{"keep inside the range", 17, 5, 3, 12, 20},
		{"lead clamped to to", 19, 5, 3, 14, 20},
		{"tip at to", 20, 5, 3, 15, 20},
		{"nothing kept", 12, 0, 1, 12, 13},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &config{from: 10, to: 20, keep: test.keep, lead: test.lead}
			snapshot := newSnapshot(advance{tip: test.tip})
			lo, hi := snapshot.bounds(config)
			require.Equal(t, test.wantLo, lo)
			require.Equal(t, test.wantHi, hi)
		})
	}
}

func TestSnapshotPhase(t *testing.T) {
	interval := 4 * time.Second
	tests := []struct {
		name     string
		interval time.Duration
		stages   uint64
		elapsed  time.Duration
		want     uint64
	}{
		{"no interval reveals everything", 0, 3, 0, 3},
		{"start", interval, 3, 0, 0},
		{"just before a stage", interval, 3, time.Second - 1, 0},
		{"on a stage", interval, 3, time.Second, 1},
		{"last stage", interval, 3, 3 * time.Second, 3},
		{"past the interval", interval, 3, time.Minute, 3},
		{"clock went backwards", interval, 3, -time.Second, 0},
		{"no stages", interval, 0, 2 * time.Second, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := newSnapshot(advance{interval: test.interval})
			require.Equal(t, test.want, snapshot.phase(snapshot.start.Add(test.elapsed), test.stages))
		})
	}
}

func TestResolveBlockNumber(t *testing.T) {
	tests := []struct {
		name        string
		tip         uint64
		blockNumber string
		wantNumber  uint64
		wantLatest  bool
		wantErr     error
	}{
		{"latest is the top block", 15, latestBlock, 18, true, nil},
		{"lowest kept", 15, "13", 13, false, nil},
		{"tip", 15, "15", 15, false, nil},
		{"top", 15, "18", 18, false, nil},
		{"latest clamped to to", 19, latestBlock, 20, true, nil},
		{
			"below the window", 15, "12", 0, false,
			notFoundf("Pre-confirmed block with number 12 was not found."),
		},
		{
			"above the window", 15, "19", 0, false,
			notFoundf("Pre-confirmed block with number 19 was not found."),
		},
		{"latest at to", 20, latestBlock, 0, false, notFoundf("No pre-confirmed block.")},
		{"missing", 15, "", 0, false, malformedf("Field blockNumber is required.")},
		{
			"not a number", 15, "abc", 0, false,
			malformedf(`get_preconfirmed_block: strconv.ParseUint: parsing "abc": invalid syntax`),
		},
		{
			"negative", 15, "-1", 0, false,
			malformedf(`get_preconfirmed_block: strconv.ParseUint: parsing "-1": invalid syntax`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &config{from: 10, to: 20, keep: 2, lead: 3}
			snapshot := newSnapshot(advance{tip: test.tip})
			number, latest, err := snapshot.resolveBlockNumber(test.blockNumber, config)
			require.Equal(t, test.wantErr, err)
			require.Equal(t, test.wantNumber, number)
			require.Equal(t, test.wantLatest, latest)
		})
	}
}

func TestRevealed(t *testing.T) {
	tests := []struct {
		name                 string
		total, stages, phase uint64
		want                 uint64
	}{
		{"no stages", 45, 0, 0, 45},
		{"first phase", 45, 3, 0, 0},
		{"second phase", 45, 3, 1, 15},
		{"third phase", 45, 3, 2, 30},
		{"last phase", 45, 3, 3, 45},
		{"rounds down", 40, 3, 1, 13},
		{"empty block", 0, 3, 2, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, revealed(test.total, test.stages, test.phase))
		})
	}
}

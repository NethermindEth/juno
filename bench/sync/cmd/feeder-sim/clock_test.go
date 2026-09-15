package main

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

func timestamps(values ...uint64) []blockInfo {
	blocks := make([]blockInfo, len(values))
	for index, value := range values {
		blocks[index] = blockInfo{timestamp: value}
	}
	return blocks
}

func TestNextDelay(t *testing.T) {
	tests := []struct {
		name     string
		blocks   []blockInfo
		from     uint64
		tip      uint64
		interval time.Duration
		speed    float64
		want     time.Duration
	}{
		{"fixed interval", timestamps(0, 1000), 0, 0, 2 * time.Second, 0, 2 * time.Second},
		{"real time", timestamps(100, 110), 0, 0, time.Second, 1, 10 * time.Second},
		{"four times faster", timestamps(100, 110), 0, 0, time.Second, 4, 2500 * time.Millisecond},
		{"half speed", timestamps(100, 110), 0, 0, time.Second, 0.5, 20 * time.Second},
		{"equal timestamps", timestamps(100, 100), 0, 0, time.Second, 1, 0},
		{"clock went backwards", timestamps(100, 90), 0, 0, time.Second, 1, 0},
		{"tip offset by from", timestamps(0, 100, 130), 10, 11, time.Second, 1, 30 * time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &config{from: test.from, to: test.from + uint64(len(test.blocks)) - 1, tip: test.tip}
			config.interval = test.interval
			config.speed = test.speed
			clock := newClock(test.blocks, config, log.NewNopZapLogger())
			require.Equal(t, test.tip, clock.tip())
			require.Equal(t, test.want, clock.nextDelay())
		})
	}
}

func TestClockRunAdvancesToEnd(t *testing.T) {
	tests := []struct {
		name  string
		speed float64
	}{
		{"fixed interval", 0},
		{"replay", 1000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig(10, 14, 11)
			config.speed = test.speed
			clock := newClock(timestamps(0, 0, 1, 1, 2), config, log.NewNopZapLogger())

			clock.run(t.Context())
			require.Equal(t, uint64(14), clock.tip())
		})
	}
}

func TestClockRunStopsOnCancel(t *testing.T) {
	config := testConfig(0, 5, 2)
	config.interval = time.Hour
	clock := newClock(timestamps(0, 0, 0, 0, 0, 0), config, log.NewNopZapLogger())

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	clock.run(ctx)
	require.Equal(t, uint64(2), clock.tip())
}

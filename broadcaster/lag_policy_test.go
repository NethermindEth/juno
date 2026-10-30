package broadcaster_test

import (
	"iter"
	"testing"

	"github.com/NethermindEth/juno/broadcaster"
	"github.com/NethermindEth/juno/broadcaster/broadcast/ring"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// fakeStream produces a sequence of EventOrLag values for testing transforms.
func fakeStream[T any](items ...ring.EventOrLag[T]) iter.Seq[ring.EventOrLag[T]] {
	return func(yield func(ring.EventOrLag[T]) bool) {
		for _, it := range items {
			if !yield(it) {
				return
			}
		}
	}
}

func collect[T any](seq iter.Seq[T]) []T {
	out := []T{}
	for v := range seq {
		out = append(out, v)
	}
	return out
}

// TestLagPolicyDrop verifies events flow through and lag entries are silently skipped.
func TestLagPolicyDrop(t *testing.T) {
	stream := fakeStream(
		ring.NewEvent(1),
		ring.NewLag[int](5, 10),
		ring.NewEvent(2),
		ring.NewEvent(3),
	)

	got := collect(broadcaster.LagPolicyDrop(stream))
	assert.Equal(t, []int{1, 2, 3}, got)
}

// TestLagPolicyLog verifies events flow through and each lag emits exactly one warn line
// carrying the missed and next-available sequence numbers.
func TestLagPolicyLog(t *testing.T) {
	loggerCore, observed := observer.New(zapcore.WarnLevel)
	zlog := log.NewZapLoggerWithCore(loggerCore)

	stream := fakeStream(
		ring.NewEvent(1),
		ring.NewLag[int](5, 10),
		ring.NewEvent(2),
	)

	transform := broadcaster.LagPolicyLog[int](zlog)
	got := collect(transform(stream))

	assert.Equal(t, []int{1, 2}, got)
	entries := observed.All()
	if assert.Len(t, entries, 1) {
		assert.Equal(t, "broadcaster subscriber lagged", entries[0].Message)
		ctx := entries[0].ContextMap()
		assert.Equal(t, uint64(5), ctx["missedSeq"])
		assert.Equal(t, uint64(10), ctx["nextSeq"])
	}
}

func blockWithNumber(number uint64) *core.Block {
	return &core.Block{Header: &core.Header{Number: number}}
}

func blockEvent(number uint64) ring.EventOrLag[*core.Block] {
	return ring.NewEvent(blockWithNumber(number))
}

func lagEvent(missedSeq, nextSeq uint64) ring.EventOrLag[*core.Block] {
	return ring.NewLag[*core.Block](missedSeq, nextSeq)
}

// fakeBlockReader serves blocks whose numbers were registered; any other number
// returns db.ErrKeyNotFound, mirroring a block that is not yet persisted.
type fakeBlockReader struct {
	available map[uint64]*core.Block
}

func newFakeBlockReader(numbers ...uint64) fakeBlockReader {
	available := make(map[uint64]*core.Block, len(numbers))
	for _, number := range numbers {
		available[number] = blockWithNumber(number)
	}
	return fakeBlockReader{available: available}
}

func (f fakeBlockReader) BlockByNumber(number uint64) (*core.Block, error) {
	if block, ok := f.available[number]; ok {
		return block, nil
	}
	return nil, db.ErrKeyNotFound
}

func numbersOf(blocks []*core.Block) []uint64 {
	numbers := make([]uint64, len(blocks))
	for i, block := range blocks {
		numbers[i] = block.Number
	}
	return numbers
}

func TestLagPolicyBlockReplay(t *testing.T) {
	tests := []struct {
		name      string
		stream    []ring.EventOrLag[*core.Block]
		available []uint64 // block numbers recoverable from the db
		want      []uint64 // expected yielded block numbers, in order
	}{
		{
			name:   "steady stream forwards events, no recovery",
			stream: []ring.EventOrLag[*core.Block]{blockEvent(1), blockEvent(2), blockEvent(3)},
			want:   []uint64{1, 2, 3},
		},
		{
			name:      "single block gap recovered",
			stream:    []ring.EventOrLag[*core.Block]{blockEvent(5), lagEvent(6, 7), blockEvent(7)},
			available: []uint64{6},
			want:      []uint64{5, 6, 7},
		},
		{
			name:      "multi block gap recovered in order",
			stream:    []ring.EventOrLag[*core.Block]{blockEvent(10), lagEvent(11, 14), blockEvent(14)},
			available: []uint64{11, 12, 13},
			want:      []uint64{10, 11, 12, 13, 14},
		},
		{
			name:      "genesis anchor recovers following gap",
			stream:    []ring.EventOrLag[*core.Block]{blockEvent(0), lagEvent(1, 3), blockEvent(3)},
			available: []uint64{1, 2},
			want:      []uint64{0, 1, 2, 3},
		},
		{
			name:      "lag before first event is anchored on that event",
			stream:    []ring.EventOrLag[*core.Block]{lagEvent(1, 6), blockEvent(6), blockEvent(7)},
			available: []uint64{1, 2, 3, 4, 5},
			want:      []uint64{1, 2, 3, 4, 5, 6, 7},
		},
		{
			name: "back to back lags before first event accumulate",
			stream: []ring.EventOrLag[*core.Block]{
				lagEvent(1, 3), lagEvent(3, 5), blockEvent(5),
			},
			available: []uint64{1, 2, 3, 4},
			want:      []uint64{1, 2, 3, 4, 5},
		},
		{
			name:      "reorg in pre-anchor gap clamps replay to genesis",
			stream:    []ring.EventOrLag[*core.Block]{lagEvent(1, 10), blockEvent(3)},
			available: []uint64{0, 1, 2},
			want:      []uint64{0, 1, 2, 3},
		},
		{
			name: "pending lag then later lag both recovered",
			stream: []ring.EventOrLag[*core.Block]{
				lagEvent(1, 3), blockEvent(3), lagEvent(4, 6), blockEvent(6),
			},
			available: []uint64{1, 2, 4, 5},
			want:      []uint64{1, 2, 3, 4, 5, 6},
		},
		{
			name:      "unrecoverable block skipped, alignment preserved",
			stream:    []ring.EventOrLag[*core.Block]{blockEvent(5), lagEvent(6, 9), blockEvent(9)},
			available: []uint64{6, 8}, // 7 missing
			want:      []uint64{5, 6, 8, 9},
		},
		{
			name: "back to back lags",
			stream: []ring.EventOrLag[*core.Block]{
				blockEvent(5), lagEvent(6, 8), lagEvent(8, 10), blockEvent(10),
			},
			available: []uint64{6, 7, 8, 9},
			want:      []uint64{5, 6, 7, 8, 9, 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.NewZapLoggerWithCore(zapcore.NewNopCore())
			policy := broadcaster.LagPolicyBlockReplay(newFakeBlockReader(tt.available...), logger)

			got := collect(policy(fakeStream(tt.stream...)))

			assert.Equal(t, tt.want, numbersOf(got))
		})
	}
}

// TestLagPolicyBlockReplayLogsUnrecoverable verifies the lag warning fires and each
// block that cannot be recovered emits its own warn line carrying the block number,
// while the anchor still advances so the live stream stays aligned.
func TestLagPolicyBlockReplayLogsUnrecoverable(t *testing.T) {
	loggerCore, observed := observer.New(zapcore.WarnLevel)
	logger := log.NewZapLoggerWithCore(loggerCore)

	// Block 7 is missing from the db; 6 is present.
	policy := broadcaster.LagPolicyBlockReplay(newFakeBlockReader(6), logger)

	stream := fakeStream(blockEvent(5), lagEvent(6, 8), blockEvent(8))
	got := collect(policy(stream))

	assert.Equal(t, []uint64{5, 6, 8}, numbersOf(got))

	entries := observed.All()
	if assert.Len(t, entries, 2) {
		assert.Equal(t, "broadcaster subscriber lagged; recovering missed blocks from db",
			entries[0].Message)
		assert.Equal(t, "block replay could not recover block", entries[1].Message)
		assert.Equal(t, uint64(7), entries[1].ContextMap()["number"])
	}
}

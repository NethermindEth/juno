package blockchain_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func TestEventMatcher_MatchesEventKeys(t *testing.T) {
	testCases := []struct {
		name       string
		filterKeys [][]felt.Felt
		eventKeys  []felt.Felt
		expected   bool
	}{
		{
			name:       "exact match single key",
			filterKeys: [][]felt.Felt{{felt.FromUint64[felt.Felt](1)}},
			eventKeys:  []felt.Felt{felt.FromUint64[felt.Felt](1)},
			expected:   true,
		},
		{
			name:       "no match single key",
			filterKeys: [][]felt.Felt{{felt.FromUint64[felt.Felt](1)}},
			eventKeys:  []felt.Felt{felt.FromUint64[felt.Felt](2)},
			expected:   false,
		},
		{
			name: "multiple positions exact match",
			filterKeys: [][]felt.Felt{
				{felt.FromUint64[felt.Felt](1)},
				{felt.FromUint64[felt.Felt](2)},
				{felt.FromUint64[felt.Felt](3)},
			},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](1),
				felt.FromUint64[felt.Felt](2),
				felt.FromUint64[felt.Felt](3),
			},
			expected: true,
		},
		{
			name: "multiple positions partial match",
			filterKeys: [][]felt.Felt{
				{felt.FromUint64[felt.Felt](1)},
				{felt.FromUint64[felt.Felt](2)},
				{felt.FromUint64[felt.Felt](3)},
			},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](1),
				felt.FromUint64[felt.Felt](99),
				felt.FromUint64[felt.Felt](3),
			},
			expected: false,
		},
		{
			name: "empty filter position matches any",
			filterKeys: [][]felt.Felt{
				{felt.FromUint64[felt.Felt](1)},
				{},
				{felt.FromUint64[felt.Felt](3)},
			},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](1),
				felt.FromUint64[felt.Felt](999),
				felt.FromUint64[felt.Felt](3),
			},
			expected: true,
		},
		{
			name: "empty filter position but wrong other positions",
			filterKeys: [][]felt.Felt{
				{felt.FromUint64[felt.Felt](1)},
				{},
				{felt.FromUint64[felt.Felt](3)},
			},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](1),
				felt.FromUint64[felt.Felt](999),
				felt.FromUint64[felt.Felt](99),
			},
			expected: false,
		},
		{
			name: "event has more keys than filter",
			filterKeys: [][]felt.Felt{
				{felt.FromUint64[felt.Felt](1)},
				{felt.FromUint64[felt.Felt](2)},
			},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](1),
				felt.FromUint64[felt.Felt](2),
				felt.FromUint64[felt.Felt](3),
				felt.FromUint64[felt.Felt](4),
			},
			expected: true,
		},
		{
			name: "event has fewer keys than filter",
			filterKeys: [][]felt.Felt{
				{felt.FromUint64[felt.Felt](1)},
				{felt.FromUint64[felt.Felt](2)},
				{},
			},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](1),
				felt.FromUint64[felt.Felt](2),
			},
			expected: false,
		},
		{
			name: "multiple values at position - first matches",
			filterKeys: [][]felt.Felt{
				{
					felt.FromUint64[felt.Felt](1),
					felt.FromUint64[felt.Felt](2),
					felt.FromUint64[felt.Felt](3),
				},
			},
			eventKeys: []felt.Felt{felt.FromUint64[felt.Felt](1)},
			expected:  true,
		},
		{
			name: "multiple values at position - second matches",
			filterKeys: [][]felt.Felt{
				{
					felt.FromUint64[felt.Felt](1),
					felt.FromUint64[felt.Felt](2),
					felt.FromUint64[felt.Felt](3),
				},
			},
			eventKeys: []felt.Felt{felt.FromUint64[felt.Felt](2)},
			expected:  true,
		},
		{
			name: "multiple values at position - none match",
			filterKeys: [][]felt.Felt{
				{
					felt.FromUint64[felt.Felt](1),
					felt.FromUint64[felt.Felt](2),
					felt.FromUint64[felt.Felt](3),
				},
			},
			eventKeys: []felt.Felt{felt.FromUint64[felt.Felt](99)},
			expected:  false,
		},
		{
			name:       "empty filter matches all events",
			filterKeys: [][]felt.Felt{},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](1),
				felt.FromUint64[felt.Felt](2),
				felt.FromUint64[felt.Felt](3),
			},
			expected: true,
		},
		{
			name:       "empty event with empty filter",
			filterKeys: [][]felt.Felt{},
			eventKeys:  []felt.Felt{},
			expected:   true,
		},
		{
			name:       "empty event with non-empty filter",
			filterKeys: [][]felt.Felt{{felt.FromUint64[felt.Felt](1)}},
			eventKeys:  []felt.Felt{},
			expected:   false,
		},
		{
			name: "complex filter with OR logic",
			filterKeys: [][]felt.Felt{
				{felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](2)},
				{},
				{felt.FromUint64[felt.Felt](5), felt.FromUint64[felt.Felt](6)},
			},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](2),
				felt.FromUint64[felt.Felt](999),
				felt.FromUint64[felt.Felt](5),
			},
			expected: true,
		},
		{
			name: "complex filter with OR logic - no match at last position",
			filterKeys: [][]felt.Felt{
				{felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](2)},
				{},
				{felt.FromUint64[felt.Felt](5), felt.FromUint64[felt.Felt](6)},
			},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](2),
				felt.FromUint64[felt.Felt](999),
				felt.FromUint64[felt.Felt](99),
			},
			expected: false,
		},
		{
			name:       "all positions empty except last",
			filterKeys: [][]felt.Felt{{}, {}, {felt.FromUint64[felt.Felt](3)}},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](1),
				felt.FromUint64[felt.Felt](2),
				felt.FromUint64[felt.Felt](3),
			},
			expected: true,
		},
		{
			name:       "all positions empty except last - wrong value",
			filterKeys: [][]felt.Felt{{}, {}, {felt.FromUint64[felt.Felt](3)}},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](1),
				felt.FromUint64[felt.Felt](2),
				felt.FromUint64[felt.Felt](99),
			},
			expected: false,
		},
		{
			name: "event shorter than filter with empty at end",
			filterKeys: [][]felt.Felt{
				{},
				{},
				{felt.FromUint64[felt.Felt](3)},
			},
			eventKeys: []felt.Felt{
				felt.FromUint64[felt.Felt](1),
				felt.FromUint64[felt.Felt](2),
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matcher := blockchain.NewEventMatcher(nil, tc.filterKeys)
			result := matcher.MatchesEventKeys(tc.eventKeys)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestEventMatcher_AppendBlockEvents(t *testing.T) {
	emitter := felt.NewFromUint64[felt.Felt](0xa)
	otherEmitter := felt.NewFromUint64[felt.Felt](0xb)
	blockHash := felt.NewFromUint64[felt.Felt](0xc0ffee)

	// Two transactions, two events each; all four from emitter.
	blockEvents := []core.TransactionEvents{
		{
			TransactionHash: felt.NewFromUint64[felt.Felt](0x1),
			Events: []*core.Event{
				{From: emitter, Keys: []felt.Felt{felt.FromUint64[felt.Felt](0x11)}},
				{From: emitter, Keys: []felt.Felt{felt.FromUint64[felt.Felt](0x12)}},
			},
		},
		{
			TransactionHash: felt.NewFromUint64[felt.Felt](0x2),
			Events: []*core.Event{
				{From: emitter, Keys: []felt.Felt{felt.FromUint64[felt.Felt](0x21)}},
				{From: emitter, Keys: []felt.Felt{felt.FromUint64[felt.Felt](0x22)}},
			},
		},
	}

	countingHashFn := func(calls *int) func() (*felt.Felt, error) {
		return func() (*felt.Felt, error) {
			*calls++
			return blockHash, nil
		}
	}

	t.Run("no match resolves no hash", func(t *testing.T) {
		matcher := blockchain.NewEventMatcher([]felt.Address{felt.Address(*otherEmitter)}, nil)
		calls := 0
		matched, processed, err := matcher.AppendBlockEvents(
			nil, 1, countingHashFn(&calls), blockEvents, 0, 10,
		)
		require.NoError(t, err)
		require.Empty(t, matched)
		require.Equal(t, uint64(4), processed)
		require.Zero(t, calls, "block hash must not be resolved when nothing matches")
	})

	t.Run("matches resolve the hash once and stamp every event", func(t *testing.T) {
		matcher := blockchain.NewEventMatcher(nil, nil)
		calls := 0
		matched, processed, err := matcher.AppendBlockEvents(
			nil, 1, countingHashFn(&calls), blockEvents, 0, 10,
		)
		require.NoError(t, err)
		require.Len(t, matched, 4)
		require.Equal(t, uint64(4), processed)
		require.Equal(t, 1, calls, "block hash must be resolved exactly once per block")
		for i, event := range matched {
			require.Equal(t, blockHash, event.BlockHash)
			require.Equal(t, blockEvents[i/2].TransactionHash, event.TransactionHash)
			require.Equal(t, uint(i/2), event.TransactionIndex)
			require.Equal(t, uint(i%2), event.EventIndex)
		}
	})

	t.Run("hash resolution failure surfaces", func(t *testing.T) {
		matcher := blockchain.NewEventMatcher(nil, nil)
		hashErr := errors.New("header gone")
		matched, processed, err := matcher.AppendBlockEvents(
			nil, 1, func() (*felt.Felt, error) { return nil, hashErr }, blockEvents, 0, 10,
		)
		require.ErrorIs(t, err, hashErr)
		require.Nil(t, matched)
		require.Zero(t, processed)
	})

	t.Run("chunk limit stops mid-block and resumes via skipped events", func(t *testing.T) {
		matcher := blockchain.NewEventMatcher(nil, nil)
		calls := 0
		firstPage, processed, err := matcher.AppendBlockEvents(
			nil, 1, countingHashFn(&calls), blockEvents, 0, 2,
		)
		// errChunkSizeReached is not exported. Therefore compare the message and do
		// not use ErrorIs.
		require.EqualError(t, err, "chunk size reached")
		require.Len(t, firstPage, 2)
		require.Equal(t, uint64(2), processed)

		calls = 0
		secondPage, processed, err := matcher.AppendBlockEvents(
			nil, 1, countingHashFn(&calls), blockEvents, processed, 10,
		)
		require.NoError(t, err)
		require.Len(t, secondPage, 2)
		require.Equal(t, uint64(4), processed)
		require.Equal(t, 1, calls)
		require.Equal(t, uint(1), secondPage[0].TransactionIndex)
		require.Equal(t, uint(0), secondPage[0].EventIndex)
	})
}

package blockchain_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func TestEventMatcher_MatchesEventKeys(t *testing.T) {
	testCases := []struct {
		name       string
		filterKeys [][]felt.Felt
		eventKeys  []*felt.Felt
		expected   bool
	}{
		{
			name:       "exact match single key",
			filterKeys: [][]felt.Felt{{felt.FromUint64[felt.Felt](1)}},
			eventKeys:  []*felt.Felt{felt.NewFromUint64[felt.Felt](1)},
			expected:   true,
		},
		{
			name:       "no match single key",
			filterKeys: [][]felt.Felt{{felt.FromUint64[felt.Felt](1)}},
			eventKeys:  []*felt.Felt{felt.NewFromUint64[felt.Felt](2)},
			expected:   false,
		},
		{
			name: "multiple positions exact match",
			filterKeys: [][]felt.Felt{
				{felt.FromUint64[felt.Felt](1)},
				{felt.FromUint64[felt.Felt](2)},
				{felt.FromUint64[felt.Felt](3)},
			},
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](2),
				felt.NewFromUint64[felt.Felt](3),
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
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](99),
				felt.NewFromUint64[felt.Felt](3),
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
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](999),
				felt.NewFromUint64[felt.Felt](3),
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
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](999),
				felt.NewFromUint64[felt.Felt](99),
			},
			expected: false,
		},
		{
			name: "event has more keys than filter",
			filterKeys: [][]felt.Felt{
				{felt.FromUint64[felt.Felt](1)},
				{felt.FromUint64[felt.Felt](2)},
			},
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](2),
				felt.NewFromUint64[felt.Felt](3),
				felt.NewFromUint64[felt.Felt](4),
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
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](2),
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
			eventKeys: []*felt.Felt{felt.NewFromUint64[felt.Felt](1)},
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
			eventKeys: []*felt.Felt{felt.NewFromUint64[felt.Felt](2)},
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
			eventKeys: []*felt.Felt{felt.NewFromUint64[felt.Felt](99)},
			expected:  false,
		},
		{
			name:       "empty filter matches all events",
			filterKeys: [][]felt.Felt{},
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](2),
				felt.NewFromUint64[felt.Felt](3),
			},
			expected: true,
		},
		{
			name:       "empty event with empty filter",
			filterKeys: [][]felt.Felt{},
			eventKeys:  []*felt.Felt{},
			expected:   true,
		},
		{
			name:       "empty event with non-empty filter",
			filterKeys: [][]felt.Felt{{felt.FromUint64[felt.Felt](1)}},
			eventKeys:  []*felt.Felt{},
			expected:   false,
		},
		{
			name: "complex filter with OR logic",
			filterKeys: [][]felt.Felt{
				{felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](2)},
				{},
				{felt.FromUint64[felt.Felt](5), felt.FromUint64[felt.Felt](6)},
			},
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](2),
				felt.NewFromUint64[felt.Felt](999),
				felt.NewFromUint64[felt.Felt](5),
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
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](2),
				felt.NewFromUint64[felt.Felt](999),
				felt.NewFromUint64[felt.Felt](99),
			},
			expected: false,
		},
		{
			name:       "all positions empty except last",
			filterKeys: [][]felt.Felt{{}, {}, {felt.FromUint64[felt.Felt](3)}},
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](2),
				felt.NewFromUint64[felt.Felt](3),
			},
			expected: true,
		},
		{
			name:       "all positions empty except last - wrong value",
			filterKeys: [][]felt.Felt{{}, {}, {felt.FromUint64[felt.Felt](3)}},
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](2),
				felt.NewFromUint64[felt.Felt](99),
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
			eventKeys: []*felt.Felt{
				felt.NewFromUint64[felt.Felt](1),
				felt.NewFromUint64[felt.Felt](2),
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

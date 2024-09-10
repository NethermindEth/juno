package starknet

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSegmentLengthsUnmarshal(t *testing.T) {
	tests := map[string]struct {
		json     string
		expected []SegmentLengths
	}{
		"flat": {
			json: "[1,2,3]",
			expected: []SegmentLengths{
				{
					Length: 1,
				},
				{
					Length: 2,
				},
				{
					Length: 3,
				},
			},
		},
		"one level nesting": {
			json: "[1,[2,3]]",
			expected: []SegmentLengths{
				{
					Length: 1,
				},
				{
					Children: []SegmentLengths{
						{
							Length: 2,
						},
						{
							Length: 3,
						},
					},
				},
			},
		},
		"multiple level nesting": {
			json: "[1,[2,3],[4,[5,6]]]",
			expected: []SegmentLengths{
				{
					Length: 1,
				},
				{
					Children: []SegmentLengths{
						{
							Length: 2,
						},
						{
							Length: 3,
						},
					},
				},
				{
					Children: []SegmentLengths{
						{
							Length: 4,
						},
						{
							Children: []SegmentLengths{
								{
									Length: 5,
								},
								{
									Length: 6,
								},
							},
						},
					},
				},
			},
		},
	}

	for desc, test := range tests {
		t.Run(desc, func(t *testing.T) {
			var unmarshaled []SegmentLengths
			require.NoError(t, json.Unmarshal([]byte(test.json), &unmarshaled))
			assert.Equal(t, test.expected, unmarshaled)

			marshaledJSON, err := json.Marshal(test.expected)
			require.NoError(t, err)
			require.Equal(t, test.json, string(marshaledJSON))
		})
	}
}

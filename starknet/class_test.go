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
			json: "[1 ,2 ,3]",
			expected: []SegmentLengths{
				SegmentLengths{
					Length: 1,
				},
				SegmentLengths{
					Length: 2,
				},
				SegmentLengths{
					Length: 3,
				},
			},
		},
		"one level nesting": {
			json: "[1 ,[2 ,3]]",
			expected: []SegmentLengths{
				SegmentLengths{
					Length: 1,
				},
				SegmentLengths{
					Children: []SegmentLengths{
						SegmentLengths{
							Length: 2,
						},
						SegmentLengths{
							Length: 3,
						},
					},
				},
			},
		},
		"multiple level nesting": {
			json: "[1 ,[2 ,3], [4, [5, 6]]]",
			expected: []SegmentLengths{
				SegmentLengths{
					Length: 1,
				},
				SegmentLengths{
					Children: []SegmentLengths{
						SegmentLengths{
							Length: 2,
						},
						SegmentLengths{
							Length: 3,
						},
					},
				},
				SegmentLengths{
					Children: []SegmentLengths{
						SegmentLengths{
							Length: 4,
						},
						SegmentLengths{
							Children: []SegmentLengths{
								SegmentLengths{
									Length: 5,
								},
								SegmentLengths{
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
		})
	}
}

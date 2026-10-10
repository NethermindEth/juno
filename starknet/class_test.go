package starknet

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntryPointOffset(t *testing.T) {
	t.Run("unmarshal decimal integer", func(t *testing.T) {
		var o EntryPointOffset
		require.NoError(t, json.Unmarshal([]byte(`161`), &o))
		assert.Equal(t, "0xa1", o.String())
	})

	t.Run("unmarshal hex string", func(t *testing.T) {
		var o EntryPointOffset
		require.NoError(t, json.Unmarshal([]byte(`"0xa1"`), &o))
		assert.Equal(t, "0xa1", o.String())
	})

	t.Run("unmarshal zero", func(t *testing.T) {
		var o EntryPointOffset
		require.NoError(t, json.Unmarshal([]byte(`0`), &o))
		assert.Equal(t, "0x0", o.String())
	})

	t.Run("unmarshal invalid", func(t *testing.T) {
		var o EntryPointOffset
		assert.Error(t, json.Unmarshal([]byte(`"notahex"`), &o))
	})

	t.Run("marshal roundtrip decimal", func(t *testing.T) {
		var o EntryPointOffset
		require.NoError(t, json.Unmarshal([]byte(`161`), &o))
		b, err := json.Marshal(o)
		require.NoError(t, err)
		assert.Equal(t, `"0xa1"`, string(b))
	})

	t.Run("marshal roundtrip hex string", func(t *testing.T) {
		var o EntryPointOffset
		require.NoError(t, json.Unmarshal([]byte(`"0xa1"`), &o))
		b, err := json.Marshal(o)
		require.NoError(t, err)
		assert.Equal(t, `"0xa1"`, string(b))
	})

	t.Run("append text", func(t *testing.T) {
		var o EntryPointOffset
		require.NoError(t, json.Unmarshal([]byte(`"0xa1"`), &o))

		appended, err := o.AppendText([]byte("prefix:"))
		require.NoError(t, err)
		assert.Equal(t, "prefix:0xa1", string(appended))

		text, err := o.MarshalText()
		require.NoError(t, err)
		assert.Equal(t, "prefix:"+string(text), string(appended))
	})

	// MarshalText has to keep a value receiver: encoding/json cannot call a
	// pointer-receiver method on a non-addressable value, and would silently fall
	// back to encoding the underlying [4]uint64 limbs as a JSON array.
	t.Run("value field in struct value", func(t *testing.T) {
		var o EntryPointOffset
		require.NoError(t, json.Unmarshal([]byte(`"0xa1"`), &o))

		var result any = struct {
			V EntryPointOffset `json:"v"`
		}{V: o}

		got, err := json.Marshal(result)
		require.NoError(t, err)
		assert.JSONEq(t, `{"v":"0xa1"}`, string(got))
	})
}

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

func TestSegmentLengthsMarshal(t *testing.T) {
	inputs := []string{"[1,2,3]", "[1,[2,3]]", "[[1],[2,[3,4]]]"}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			var segments []SegmentLengths
			require.NoError(t, json.Unmarshal([]byte(input), &segments))
			data, err := json.Marshal(segments)
			require.NoError(t, err)
			assert.JSONEq(t, input, string(data))
		})
	}
}

func TestClassDefinitionUnmarshal(t *testing.T) {
	t.Run("sierra", func(t *testing.T) {
		var class ClassDefinition
		require.NoError(t, json.Unmarshal([]byte(`{
			"abi":"[]",
			"entry_points_by_type":{"CONSTRUCTOR":[],"EXTERNAL":[],"L1_HANDLER":[]},
			"sierra_program":["0x1","0x2"],
			"contract_class_version":"0.1.0"
		}`), &class))
		require.Nil(t, class.DeprecatedCairo)
		require.NotNil(t, class.Sierra)
		assert.Len(t, class.Sierra.Program, 2)
		assert.Equal(t, "0.1.0", class.Sierra.Version)
	})

	t.Run("deprecated cairo", func(t *testing.T) {
		var class ClassDefinition
		require.NoError(t, json.Unmarshal([]byte(`{
			"abi":[{"type":"function","name":"f"}],
			"entry_points_by_type":{"CONSTRUCTOR":[],"EXTERNAL":[],"L1_HANDLER":[]},
			"program":{"data":["0x1"],"prime":"0x1"}
		}`), &class))
		require.Nil(t, class.Sierra)
		require.NotNil(t, class.DeprecatedCairo)
		assert.JSONEq(t, `[{"type":"function","name":"f"}]`, string(class.DeprecatedCairo.Abi))
		assert.Contains(t, string(class.DeprecatedCairo.Program), `"data":["0x1"]`)
	})

	t.Run("not an object", func(t *testing.T) {
		var class ClassDefinition
		require.Error(t, json.Unmarshal([]byte(`[]`), &class))
	})

	t.Run("array holding the sierra key is not a sierra class", func(t *testing.T) {
		var class ClassDefinition
		require.Error(t, json.Unmarshal([]byte(`["sierra_program", 1]`), &class))
		assert.Nil(t, class.Sierra)
	})

	t.Run("duplicate member names are tolerated like encoding/json", func(t *testing.T) {
		var class ClassDefinition
		require.NoError(t, json.Unmarshal([]byte(`{
			"abi":[],
			"abi":[],
			"entry_points_by_type":{"CONSTRUCTOR":[],"EXTERNAL":[],"L1_HANDLER":[]},
			"program":{}
		}`), &class))
		require.NotNil(t, class.DeprecatedCairo)
	})
}

func TestIsDeprecatedCompiledClassDefinition(t *testing.T) {
	tests := map[string]struct {
		json       string
		deprecated bool
	}{
		"cairo 0 program":           {`{"program":{"data":[]},"entry_points_by_type":{}}`, true},
		"null program still counts": {`{"program":null}`, true},
		"duplicate keys":            {`{"program":{},"program":{}}`, true},
		"casm":                      {`{"bytecode":[],"prime":"0x1","hints":[]}`, false},
		"null":                      {`null`, false},
		"array":                     {`[]`, false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			deprecated, err := IsDeprecatedCompiledClassDefinition(json.RawMessage(tc.json))
			require.NoError(t, err)
			assert.Equal(t, tc.deprecated, deprecated)
		})
	}

	t.Run("truncated after the program key", func(t *testing.T) {
		truncated := json.RawMessage(`{"program":{},"entry_points_by_type":{`)
		_, err := IsDeprecatedCompiledClassDefinition(truncated)
		require.Error(t, err)
	})
}

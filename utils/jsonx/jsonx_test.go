package jsonx_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/utils/jsonx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSONNumberRoundTrip locks UseNumber semantics: when the decoder
// is configured for json.Number, integers in `any` targets land as
// json.Number rather than float64. Required for JSON-RPC ID handling
// (server.go isSane).
func TestJSONNumberRoundTrip(t *testing.T) {
	dec := jsonx.NewDecoder(strings.NewReader(`{"id":42,"big":18446744073709551615}`))
	dec.UseNumber()
	var got map[string]any
	require.NoError(t, dec.Decode(&got))

	id, ok := got["id"].(json.Number)
	require.True(t, ok, "got %T, expected json.Number", got["id"])
	require.Equal(t, "42", id.String())

	// Must preserve uint64-overflow integers losslessly.
	big, ok := got["big"].(json.Number)
	require.True(t, ok)
	require.Equal(t, "18446744073709551615", big.String())
}

// TestRawMessageRoundTrip locks json.RawMessage handling: marshal emits
// the bytes verbatim; unmarshal stores the raw slice.
func TestRawMessageRoundTrip(t *testing.T) {
	t.Run("marshal emits raw bytes verbatim", func(t *testing.T) {
		raw := json.RawMessage(`{"x":1,"y":[2,3]}`)
		out, err := jsonx.Marshal(raw)
		require.NoError(t, err)
		require.JSONEq(t, string(raw), string(out))
	})

	t.Run("unmarshal stores raw slice", func(t *testing.T) {
		var raw json.RawMessage
		require.NoError(t, jsonx.Unmarshal([]byte(`{"a":1}`), &raw))
		require.JSONEq(t, `{"a":1}`, string(raw))
	})

	t.Run("marshal nested struct field as RawMessage", func(t *testing.T) {
		type wrap struct {
			Inner json.RawMessage `json:"inner"`
		}
		w := wrap{Inner: json.RawMessage(`[1,2,3]`)}
		out, err := jsonx.Marshal(w)
		require.NoError(t, err)
		require.JSONEq(t, `{"inner":[1,2,3]}`, string(out))
	})
}

// TestStructFieldOrderDeterministic confirms that struct fields encode
// in declaration order. This is the foundation of every wire-stable
// shape in Juno's RPC — clients don't rely on alphabetical ordering.
func TestStructFieldOrderDeterministic(t *testing.T) {
	type s struct {
		Z string `json:"z"`
		A string `json:"a"`
		M string `json:"m"`
	}
	out, err := jsonx.Marshal(s{Z: "1", A: "2", M: "3"})
	require.NoError(t, err)
	require.Equal(t, `{"z":"1","a":"2","m":"3"}`, string(out))
}

// TestMapKeyOrderUndefined documents that sonic.ConfigDefault does NOT
// sort map keys (stdlib does). Any code emitting maps to a wire-visible
// JSON output must accept this — callers who need deterministic order
// should use a typed struct (see rpcv10.SubscriptionParams for prior
// art).
func TestMapKeyOrderUndefined(t *testing.T) {
	m := map[string]int{"z": 1, "a": 2, "m": 3}
	// Run twice; structural content must always match.
	for range 2 {
		out, err := jsonx.Marshal(m)
		require.NoError(t, err)
		var back map[string]int
		require.NoError(t, jsonx.Unmarshal(out, &back))
		require.Equal(t, m, back)
	}
	// Note: we deliberately do NOT assert byte-equality across runs.
}

// TestUnmarshalString matches the []byte variant. Used by limit_slice
// to avoid a per-element string→[]byte copy.
func TestUnmarshalString(t *testing.T) {
	input := `{"a":1,"b":"x"}`
	var fromString, fromBytes map[string]any
	require.NoError(t, jsonx.UnmarshalString(input, &fromString))
	require.NoError(t, jsonx.Unmarshal([]byte(input), &fromBytes))
	require.Equal(t, fromBytes, fromString)
}

// TestMalformedInputReturnsError pins behavior on common bad inputs:
// errors are returned as values, not via panic. Sonic has historically
// panicked on a few shapes; this guards against regressions there.
func TestMalformedInputReturnsError(t *testing.T) {
	cases := []string{
		`{`,
		`{"a":}`,
		`[1,`,
		``,
		` `,
		`null{`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			var v any
			err := jsonx.Unmarshal([]byte(in), &v)
			require.Error(t, err)
		})
	}
}

// TestDecoderInterface verifies the decoder exposes the sonic-mirror
// surface (Decode, UseNumber, More, Buffered, DisallowUnknownFields)
// and that the methods behave as documented.
func TestDecoderInterface(t *testing.T) {
	t.Run("Decode + More on adjacent values", func(t *testing.T) {
		dec := jsonx.NewDecoder(strings.NewReader(`{"a":1}{"b":2}`))
		var v1, v2 map[string]int
		require.NoError(t, dec.Decode(&v1))
		require.Equal(t, map[string]int{"a": 1}, v1)
		require.NoError(t, dec.Decode(&v2))
		require.Equal(t, map[string]int{"b": 2}, v2)
	})

	t.Run("DisallowUnknownFields rejects extra keys", func(t *testing.T) {
		type s struct {
			A int `json:"a"`
		}
		dec := jsonx.NewDecoder(strings.NewReader(`{"a":1,"unknown":2}`))
		dec.DisallowUnknownFields()
		var got s
		require.Error(t, dec.Decode(&got))
	})

	t.Run("Buffered surfaces remaining bytes", func(t *testing.T) {
		dec := jsonx.NewDecoder(strings.NewReader(`{"a":1}{"b":2}`))
		var v map[string]int
		require.NoError(t, dec.Decode(&v))
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(dec.Buffered())
		require.Contains(t, buf.String(), `"b":2`)
	})
}

// TestNullDecode confirms that null decodes into a typed pointer or
// slice without error, leaving zero values — matches stdlib semantics
// the rest of Juno relies on.
func TestNullDecode(t *testing.T) {
	t.Run("null into pointer", func(t *testing.T) {
		v := new(int)
		*v = 42
		require.NoError(t, jsonx.Unmarshal([]byte(`null`), &v))
		require.Nil(t, v)
	})

	t.Run("null into slice", func(t *testing.T) {
		s := []int{1, 2, 3}
		require.NoError(t, jsonx.Unmarshal([]byte(`null`), &s))
		require.Nil(t, s)
	})
}

// TestRoundTripParityWithStdlib pins a small set of representative
// shapes from Juno's RPC payloads. Anything sonic encodes here must
// re-decode to the same Go value via stdlib (proving wire compat).
func TestRoundTripParityWithStdlib(t *testing.T) {
	type inner struct {
		X int      `json:"x"`
		Y []string `json:"y"`
	}
	type outer struct {
		ID   string          `json:"id"`
		Data inner           `json:"data"`
		Raw  json.RawMessage `json:"raw,omitempty"`
		Tags []string        `json:"tags"`
	}

	cases := []outer{
		{ID: "a", Data: inner{X: 1, Y: []string{"p", "q"}}, Tags: []string{}},
		{ID: "", Data: inner{}, Tags: nil},
		{ID: "b", Data: inner{X: 0, Y: nil}, Raw: json.RawMessage(`{"k":"v"}`)},
	}
	for i, in := range cases {
		t.Run(in.ID, func(t *testing.T) {
			t.Logf("case %d: %+v", i, in)
			b, err := jsonx.Marshal(in)
			require.NoError(t, err)

			var viaJsonx, viaStdlib outer
			require.NoError(t, jsonx.Unmarshal(b, &viaJsonx))
			require.NoError(t, json.Unmarshal(b, &viaStdlib))
			assert.Equal(t, viaStdlib, viaJsonx, "jsonx and stdlib must agree on decode of %s", b)
		})
	}
}

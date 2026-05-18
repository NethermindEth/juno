package jsonrpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParamsKind(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want byte
	}{
		{"nil", "", paramsKindNone},
		{"whitespace only", "  \t\r\n  ", paramsKindNone},
		{"null bare", "null", paramsKindNone},
		{"null leading whitespace", "  null", paramsKindNone},
		{"array bare", "[1,2,3]", paramsKindArray},
		{"array leading whitespace", "  \t[1]", paramsKindArray},
		{"object bare", `{"a":1}`, paramsKindObject},
		{"object leading whitespace", "\n\r {\"a\":1}", paramsKindObject},
		{"number", "42", paramsKindInvalid},
		{"string", `"hello"`, paramsKindInvalid},
		{"bool true", "true", paramsKindInvalid},
		{"bool false", "false", paramsKindInvalid},
		{"lone n", "n", paramsKindInvalid},
		{"nul (one byte short of null)", "nul", paramsKindInvalid},
		{"nope (n + non-ull)", "nope", paramsKindInvalid},
		{"nul1l (right length, wrong bytes)", "nul1", paramsKindInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := paramsKind(json.RawMessage(tc.in))
			assert.Equalf(t, tc.want, got, "paramsKind(%q)", tc.in)
		})
	}
}

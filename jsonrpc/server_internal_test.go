package jsonrpc

import (
	"encoding/json"
	"reflect"
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

func TestTypeHasValidateTag(t *testing.T) {
	type plainPrim int
	type plainStruct struct {
		A int
		B string
	}
	type withTag struct {
		A int `validate:"min=1"`
	}
	type embedsTagged struct {
		withTag
		B string
	}
	type nestedTagged struct {
		Inner withTag
	}
	type wrapPtr struct {
		P *withTag
	}
	type wrapSlice struct {
		S []withTag
	}
	type wrapMapKey struct {
		M map[withTag]string
	}
	type wrapMapVal struct {
		M map[string]withTag
	}
	type recNoTag struct {
		Next *recNoTag
		Name string
	}
	type recTagged struct {
		Next *recTagged
		A    int `validate:"min=1"`
	}

	cases := []struct {
		name string
		t    reflect.Type
		want bool
	}{
		{"nil type", nil, false},
		{"primitive", reflect.TypeFor[int](), false},
		{"primitive named", reflect.TypeFor[plainPrim](), false},
		{"plain struct no tag", reflect.TypeFor[plainStruct](), false},
		{"pointer to plain", reflect.TypeFor[*plainStruct](), false},
		{"slice of plain", reflect.TypeFor[[]plainStruct](), false},
		{"map plain→plain", reflect.TypeFor[map[string]plainStruct](), false},
		{"struct with tag", reflect.TypeFor[withTag](), true},
		{"pointer to tagged", reflect.TypeFor[*withTag](), true},
		{"slice of tagged", reflect.TypeFor[[]withTag](), true},
		{"map value tagged", reflect.TypeFor[map[string]withTag](), true},
		{"map key tagged", reflect.TypeFor[map[withTag]string](), true},
		{"embedded tagged struct", reflect.TypeFor[embedsTagged](), true},
		{"nested tagged field", reflect.TypeFor[nestedTagged](), true},
		{"wrapper pointer to tagged", reflect.TypeFor[wrapPtr](), true},
		{"wrapper slice of tagged", reflect.TypeFor[wrapSlice](), true},
		{"wrapper map-key tagged", reflect.TypeFor[wrapMapKey](), true},
		{"wrapper map-val tagged", reflect.TypeFor[wrapMapVal](), true},
		{"recursive struct, no tag", reflect.TypeFor[recNoTag](), false},
		{"recursive struct, tagged", reflect.TypeFor[recTagged](), true},
		{"pointer to recursive tagged", reflect.TypeFor[*recTagged](), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := typeHasValidateTag(tc.t, nil)
			assert.Equalf(t, tc.want, got, "typeHasValidateTag(%v)", tc.t)
		})
	}
}

package jsonrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------

type fxRequiredOnly struct {
	A int    `json:"a"`
	B string `json:"b"`
}

type fxMixed struct {
	A int    `json:"a"`
	B string `json:"b,omitempty"`
	C bool   `json:"c,omitempty"`
}

type fxAllOptional struct {
	X *int `json:"x,omitempty"`
	Y *int `json:"y,omitempty"`
}

type fxValidated struct {
	A int `json:"a" validate:"min=1"`
}

// fxMissingTag has an exported field without a `json:` tag. Used to
// verify captureFieldTags panics at registration.
type fxMissingTag struct {
	A int
}

// ---------------------------------------------------------------------
// captureFieldTags
// ---------------------------------------------------------------------

func TestCaptureFieldTags_EmptyStruct(t *testing.T) {
	plan := captureFieldTags[NoParams]()
	require.NotNil(t, plan)
	assert.Empty(t, plan.tags)
	assert.Empty(t, plan.optional)
	assert.Empty(t, plan.byName)
	assert.Equal(t, 0, plan.requiredN)
}

func TestCaptureFieldTags_RequiredOnly(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	assert.Equal(t, []string{"a", "b"}, plan.tags)
	assert.Equal(t, []bool{false, false}, plan.optional)
	assert.Equal(t, map[string]int{"a": 0, "b": 1}, plan.byName)
	assert.Equal(t, 2, plan.requiredN)
}

func TestCaptureFieldTags_MixedOptional(t *testing.T) {
	plan := captureFieldTags[fxMixed]()
	assert.Equal(t, []string{"a", "b", "c"}, plan.tags)
	assert.Equal(t, []bool{false, true, true}, plan.optional)
	assert.Equal(t, 1, plan.requiredN)
}

func TestCaptureFieldTags_AllOptional(t *testing.T) {
	plan := captureFieldTags[fxAllOptional]()
	assert.Equal(t, 0, plan.requiredN)
	assert.Equal(t, []bool{true, true}, plan.optional)
}

func TestCaptureFieldTags_PanicsOnNonStruct(t *testing.T) {
	require.PanicsWithValue(t,
		"jsonrpc: param type int must be a struct",
		func() { captureFieldTags[int]() },
	)
}

func TestCaptureFieldTags_PanicsOnMissingJSONTag(t *testing.T) {
	require.PanicsWithValue(t,
		"jsonrpc: jsonrpc.fxMissingTag.A missing `json:` tag",
		func() { captureFieldTags[fxMissingTag]() },
	)
}

// ---------------------------------------------------------------------
// decodeParams routing
// ---------------------------------------------------------------------

func TestDecodeParams_NoneNoRequired(t *testing.T) {
	plan := captureFieldTags[fxAllOptional]()
	var p fxAllOptional
	require.Nil(t, decodeParams(nil, plan, &p, nil))
	require.Nil(t, decodeParams(json.RawMessage("null"), plan, &p, nil))
	require.Nil(t, decodeParams(json.RawMessage("  \t\n"), plan, &p, nil))
}

func TestDecodeParams_NoneWithRequired(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	var p fxRequiredOnly
	e := decodeParams(nil, plan, &p, nil)
	require.NotNil(t, e)
	assert.Equal(t, InvalidParams, e.Code)
	assert.Equal(t, "missing non-optional param field", e.Data)
}

func TestDecodeParams_InvalidKind(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	var p fxRequiredOnly
	e := decodeParams(json.RawMessage("42"), plan, &p, nil)
	require.NotNil(t, e)
	assert.Equal(t, "impossible param type: check request.isSane", e.Data)
}

// ---------------------------------------------------------------------
// decodeArray
// ---------------------------------------------------------------------

func TestDecodeArray_HappyPath(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	var p fxRequiredOnly
	require.Nil(t, decodeArray(json.RawMessage(`[1,"hello"]`), plan, &p, nil))
	assert.Equal(t, 1, p.A)
	assert.Equal(t, "hello", p.B)
}

func TestDecodeArray_TooFew(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	var p fxRequiredOnly
	e := decodeArray(json.RawMessage(`[1]`), plan, &p, nil)
	require.NotNil(t, e)
	assert.Equal(t, "missing/unexpected params in list", e.Data)
}

func TestDecodeArray_TooMany(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	var p fxRequiredOnly
	e := decodeArray(json.RawMessage(`[1,"hello","extra"]`), plan, &p, nil)
	require.NotNil(t, e)
	assert.Equal(t, "missing/unexpected params in list", e.Data)
}

func TestDecodeArray_OptionalMissing(t *testing.T) {
	plan := captureFieldTags[fxMixed]()
	var p fxMixed
	// `a` is required; the two trailing optionals are absent.
	require.Nil(t, decodeArray(json.RawMessage(`[7]`), plan, &p, nil))
	assert.Equal(t, 7, p.A)
	assert.Equal(t, "", p.B)
	assert.False(t, p.C)
}

func TestDecodeArray_TypeMismatchError(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	var p fxRequiredOnly
	e := decodeArray(json.RawMessage(`["nope","hello"]`), plan, &p, nil)
	require.NotNil(t, e)
	assert.Equal(t, InvalidParams, e.Code)
	// Sonic's exact wording differs across architectures; just confirm
	// the error data is non-empty and from the unmarshal path.
	assert.NotEmpty(t, e.Data)
}

func TestDecodeArray_ValidatorPass(t *testing.T) {
	v := validator.New()
	plan := captureFieldTags[fxValidated]()
	var p fxValidated
	require.Nil(t, decodeArray(json.RawMessage(`[5]`), plan, &p, v))
}

func TestDecodeArray_ValidatorFail(t *testing.T) {
	v := validator.New()
	plan := captureFieldTags[fxValidated]()
	var p fxValidated
	e := decodeArray(json.RawMessage(`[0]`), plan, &p, v)
	require.NotNil(t, e)
	assert.Contains(t, e.Data, "failed on the 'min' tag")
}

// ---------------------------------------------------------------------
// decodeObject
// ---------------------------------------------------------------------

func TestDecodeObject_HappyPath(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	var p fxRequiredOnly
	require.Nil(t, decodeObject(json.RawMessage(`{"a":2,"b":"hi"}`), plan, &p, nil))
	assert.Equal(t, 2, p.A)
	assert.Equal(t, "hi", p.B)
}

func TestDecodeObject_MissingRequired(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	var p fxRequiredOnly
	e := decodeObject(json.RawMessage(`{"b":"hi"}`), plan, &p, nil)
	require.NotNil(t, e)
	assert.Equal(t, "missing non-optional param: a", e.Data)
}

func TestDecodeObject_UnknownKey(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	var p fxRequiredOnly
	e := decodeObject(json.RawMessage(`{"a":1,"b":"hi","junk":42}`), plan, &p, nil)
	require.NotNil(t, e)
	assert.Equal(t, "unexpected params: junk", e.Data)
}

func TestDecodeObject_MultipleUnknownKeys(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	var p fxRequiredOnly
	e := decodeObject(json.RawMessage(`{"a":1,"b":"hi","x":1,"y":2}`), plan, &p, nil)
	require.NotNil(t, e)
	// ast.Node.Properties() iterates in declaration order, so the
	// joined list reflects the JSON's key order.
	assert.Equal(t, "unexpected params: x, y", e.Data)
}

func TestDecodeObject_MissingRequiredWinsOverUnknown(t *testing.T) {
	plan := captureFieldTags[fxRequiredOnly]()
	var p fxRequiredOnly
	// Both an extra key AND a missing required: legacy precedence
	// requires the missing-required message.
	e := decodeObject(json.RawMessage(`{"b":"hi","junk":42}`), plan, &p, nil)
	require.NotNil(t, e)
	assert.Equal(t, "missing non-optional param: a", e.Data)
}

func TestDecodeObject_OptionalAbsent(t *testing.T) {
	plan := captureFieldTags[fxMixed]()
	var p fxMixed
	require.Nil(t, decodeObject(json.RawMessage(`{"a":9}`), plan, &p, nil))
	assert.Equal(t, 9, p.A)
}

func TestDecodeObject_ValidatorPass(t *testing.T) {
	v := validator.New()
	plan := captureFieldTags[fxValidated]()
	var p fxValidated
	require.Nil(t, decodeObject(json.RawMessage(`{"a":1}`), plan, &p, v))
}

func TestDecodeObject_ValidatorFail(t *testing.T) {
	v := validator.New()
	plan := captureFieldTags[fxValidated]()
	var p fxValidated
	e := decodeObject(json.RawMessage(`{"a":0}`), plan, &p, v)
	require.NotNil(t, e)
	assert.Contains(t, e.Data, "failed on the 'min' tag")
}

// ---------------------------------------------------------------------
// Register / RegisterC / RegisterH / RegisterCH
//
// These tests exercise the Method shape (Name, ParamStructType,
// pretouchTypes, Dispatch) and invoke the Dispatch closure directly to
// verify ctx threading, header propagation, and the user-fn return
// path.
// ---------------------------------------------------------------------

type sampleResp struct {
	N int `json:"n"`
}

func TestRegister_MethodShape(t *testing.T) {
	m := Register("foo", func(p *fxRequiredOnly) (sampleResp, *Error) {
		return sampleResp{N: p.A}, nil
	})
	assert.Equal(t, "foo", m.Name)
	require.NotNil(t, m.Dispatch)
	assert.Equal(t, reflect.TypeFor[fxRequiredOnly](), m.ParamStructType)
	assert.Contains(t, m.pretouchTypes, reflect.TypeFor[fxRequiredOnly]())
	assert.Contains(t, m.pretouchTypes, reflect.TypeFor[sampleResp]())
}

func TestRegister_DispatchInvokes(t *testing.T) {
	m := Register("sum", func(p *fxRequiredOnly) (sampleResp, *Error) {
		return sampleResp{N: p.A + len(p.B)}, nil
	})
	result, hdr, e := m.Dispatch(t.Context(), nil, json.RawMessage(`{"a":3,"b":"hi"}`))
	require.Nil(t, e)
	assert.Nil(t, hdr)
	assert.Equal(t, sampleResp{N: 5}, result)
}

func TestRegister_DispatchPropagatesError(t *testing.T) {
	want := &Error{Code: 42, Message: "boom"}
	m := Register("explode", func(_ *fxRequiredOnly) (sampleResp, *Error) {
		return sampleResp{}, want
	})
	_, _, got := m.Dispatch(t.Context(), nil, json.RawMessage(`{"a":1,"b":""}`))
	require.NotNil(t, got)
	assert.Same(t, want, got)
}

func TestRegister_DispatchReportsParamError(t *testing.T) {
	m := Register("p", func(_ *fxRequiredOnly) (sampleResp, *Error) {
		t.Fatal("handler should not run when params fail to decode")
		return sampleResp{}, nil
	})
	_, _, e := m.Dispatch(t.Context(), nil, json.RawMessage(`{"a":1}`))
	require.NotNil(t, e)
	assert.Equal(t, "missing non-optional param: b", e.Data)
}

func TestRegisterC_ThreadsContext(t *testing.T) {
	type ctxKey struct{}
	want := "marker-value"

	m := RegisterC("ctx", func(ctx context.Context, _ *NoParams) (string, *Error) {
		v, _ := ctx.Value(ctxKey{}).(string)
		return v, nil
	})
	ctx := context.WithValue(t.Context(), ctxKey{}, want)
	got, _, e := m.Dispatch(ctx, nil, nil)
	require.Nil(t, e)
	assert.Equal(t, want, got)
}

func TestRegisterH_ReturnsHeader(t *testing.T) {
	header := http.Header{"X-Test": []string{"value"}}
	m := RegisterH("hdr", func(_ *NoParams) (int, http.Header, *Error) {
		return 7, header, nil
	})
	result, hdr, e := m.Dispatch(t.Context(), nil, nil)
	require.Nil(t, e)
	assert.Equal(t, 7, result)
	assert.Equal(t, header, hdr)
}

func TestRegisterCH_ThreadsBoth(t *testing.T) {
	type ctxKey struct{}
	header := http.Header{"X-Trace": []string{"abc"}}

	m := RegisterCH("both",
		func(ctx context.Context, p *fxRequiredOnly) (string, http.Header, *Error) {
			ctxVal, _ := ctx.Value(ctxKey{}).(string)
			return ctxVal + ":" + p.B, header, nil
		},
	)
	ctx := context.WithValue(t.Context(), ctxKey{}, "ctx")
	result, hdr, e := m.Dispatch(ctx, nil, json.RawMessage(`{"a":1,"b":"hello"}`))
	require.Nil(t, e)
	assert.Equal(t, "ctx:hello", result)
	assert.Equal(t, header, hdr)
}

func TestRegister_DispatchValidatorIntegration(t *testing.T) {
	v := validator.New()
	called := false
	m := Register("validated", func(_ *fxValidated) (int, *Error) {
		called = true
		return 0, nil
	})
	_, _, e := m.Dispatch(t.Context(), v, json.RawMessage(`{"a":0}`))
	require.NotNil(t, e)
	assert.False(t, called, "handler must not run when validator rejects")
	assert.Contains(t, e.Data, "failed on the 'min' tag")
}

func TestRegister_AllVariants_PretouchIncludesPAndR(t *testing.T) {
	pT := reflect.TypeFor[fxRequiredOnly]()
	rT := reflect.TypeFor[sampleResp]()

	cases := []struct {
		name string
		m    Method
	}{
		{
			"Register",
			Register("a", func(_ *fxRequiredOnly) (sampleResp, *Error) {
				return sampleResp{}, nil
			}),
		},
		{
			"RegisterC",
			RegisterC("b", func(_ context.Context, _ *fxRequiredOnly) (sampleResp, *Error) {
				return sampleResp{}, nil
			}),
		},
		{
			"RegisterH",
			RegisterH("c", func(_ *fxRequiredOnly) (sampleResp, http.Header, *Error) {
				return sampleResp{}, nil, nil
			}),
		},
		{
			"RegisterCH",
			RegisterCH("d", func(_ context.Context, _ *fxRequiredOnly) (sampleResp, http.Header, *Error) {
				return sampleResp{}, nil, nil
			}),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, pT, tc.m.ParamStructType)
			assert.Contains(t, tc.m.pretouchTypes, pT)
			assert.Contains(t, tc.m.pretouchTypes, rT)
		})
	}
}

// TestRegister_ArrayReshapesToObject locks in the array-to-object
// reshape behavior of decodeArray by feeding both positional and named
// forms through Dispatch and asserting identical results.
func TestRegister_ArrayReshapesToObject(t *testing.T) {
	m := Register("eq", func(p *fxRequiredOnly) (string, *Error) {
		return strings.Repeat(p.B, p.A), nil
	})

	posRes, _, e1 := m.Dispatch(t.Context(), nil, json.RawMessage(`[3,"ab"]`))
	require.Nil(t, e1)

	namedRes, _, e2 := m.Dispatch(t.Context(), nil, json.RawMessage(`{"a":3,"b":"ab"}`))
	require.Nil(t, e2)

	assert.Equal(t, namedRes, posRes)
	assert.Equal(t, "ababab", posRes)
}

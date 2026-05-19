package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
)

type NoParams = struct{}

// fieldInfo carries the per-field metadata captured at registration so
// the hot path can decode each param's bytes directly into its slot in
// *P without an intermediate reshape buffer or whole-struct unmarshal.
type fieldInfo struct {
	name     string
	optional bool
	index    int // struct field index for reflect.Value.Field
	// useNum is true if the field's type tree reaches an interface kind,
	// where stdlib's json would otherwise decode numeric literals as
	// float64 instead of json.Number. False ⇒ json.Unmarshal is safe
	// (and saves a Decoder + Reader allocation per decode).
	useNum bool
}

// decodeInto unmarshals raw into target. Picks the allocation-free
// json.Unmarshal path unless json.Number semantics are needed for this
// field's type. Caller obtains target as a *T pointer at the struct
// field's address.
func (fi *fieldInfo) decodeInto(target any, raw json.RawMessage) error {
	if fi.useNum {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		return dec.Decode(target)
	}
	return json.Unmarshal(raw, target)
}

// regPlan is the per-method metadata captured at registration. Hot
// path reads it as plain slice / map lookups.
type regPlan struct {
	fields          []fieldInfo
	byName          map[string]int // tag name → fields index
	requiredN       int
	needsValidation bool // any field (transitively) carries a `validate:` tag
}

// captureFieldTags walks P once at registration. P must be a struct;
// every exported field must carry a `json:"name[,omitempty]"` tag.
// `omitempty`/`omitzero` flag the field as optional. Reflection only
// happens here.
func captureFieldTags[P any]() *regPlan {
	pt := reflect.TypeFor[P]()
	if pt.Kind() != reflect.Struct {
		panic(fmt.Sprintf("jsonrpc: param type %s must be a struct", pt))
	}
	plan := &regPlan{}
	if pt.NumField() == 0 {
		return plan
	}
	plan.needsValidation = typeHasValidateTag(pt, nil)
	plan.fields = make([]fieldInfo, 0, pt.NumField())
	plan.byName = make(map[string]int, pt.NumField())
	for i := range pt.NumField() {
		f := pt.Field(i)
		tag, ok := f.Tag.Lookup("json")
		if !ok {
			panic(fmt.Sprintf("jsonrpc: %s.%s missing `json:` tag", pt, f.Name))
		}
		name, optsRaw, _ := strings.Cut(tag, ",")
		opts := strings.Split(optsRaw, ",")
		optional := slices.Contains(opts, "omitempty") || slices.Contains(opts, "omitzero")
		plan.byName[name] = len(plan.fields)
		plan.fields = append(plan.fields, fieldInfo{
			name:     name,
			optional: optional,
			index:    i,
			useNum:   typeNeedsUseNumber(f.Type, nil),
		})
		if !optional {
			plan.requiredN++
		}
	}
	return plan
}

// typeHasValidateTag reports whether t — or anything reachable through
// its fields, slice/array elements, map values, or pointer indirection —
// carries a `validate:` struct tag. Used at registration to decide
// whether the validator walk runs on the hot path. Cycles are guarded
// via the visited set.
func typeHasValidateTag(t reflect.Type, visited map[reflect.Type]bool) bool {
	if t == nil {
		return false
	}
	if visited == nil {
		visited = map[reflect.Type]bool{}
	}
	if visited[t] {
		return false
	}
	visited[t] = true
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return typeHasValidateTag(t.Elem(), visited)
	case reflect.Struct:
		for i := range t.NumField() {
			f := t.Field(i)
			if _, ok := f.Tag.Lookup("validate"); ok {
				return true
			}
			if typeHasValidateTag(f.Type, visited) {
				return true
			}
		}
	}
	return false
}

// typeNeedsUseNumber reports whether decoding into t requires json
// number semantics — i.e. the type tree reaches an interface kind where
// a numeric literal would otherwise lose precision by decoding to
// float64. Concrete numeric and pointer types do not need it.
func typeNeedsUseNumber(t reflect.Type, visited map[reflect.Type]bool) bool {
	if t == nil {
		return false
	}
	if visited == nil {
		visited = map[reflect.Type]bool{}
	}
	if visited[t] {
		return false
	}
	visited[t] = true
	switch t.Kind() {
	case reflect.Interface:
		return true
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return typeNeedsUseNumber(t.Elem(), visited)
	case reflect.Struct:
		for i := range t.NumField() {
			if typeNeedsUseNumber(t.Field(i).Type, visited) {
				return true
			}
		}
	}
	return false
}

// decodeParams routes raw JSON params into *p. Arity/missing/unknown
// detection produces error strings that match the JSON-RPC server's
// error format.
func decodeParams[P any](raw json.RawMessage, plan *regPlan, p *P, v Validator) *Error {
	switch paramsKind(raw) {
	case paramsKindNone:
		if plan.requiredN > 0 {
			return Err(InvalidParams, "missing non-optional param field")
		}
		return nil
	case paramsKindArray:
		return decodeArray(raw, plan, p, v)
	case paramsKindObject:
		return decodeObject(raw, plan, p, v)
	default:
		return Err(InvalidParams, "impossible param type: check request.isSane")
	}
}

// decodeArray walks the positional array and decodes each element
// directly into the corresponding slot in *p — no reshape buffer, no
// secondary whole-struct unmarshal.
func decodeArray[P any](raw json.RawMessage, plan *regPlan, p *P, v Validator) *Error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return Err(InvalidParams, err.Error())
	}
	if d, ok := tok.(json.Delim); !ok || d != '[' {
		return Err(InvalidParams, "params array expected")
	}
	pv := reflect.ValueOf(p).Elem()
	i := 0
	for dec.More() {
		if i >= len(plan.fields) {
			return Err(InvalidParams, "missing/unexpected params in list")
		}
		var elem json.RawMessage
		if err := dec.Decode(&elem); err != nil {
			return Err(InvalidParams, err.Error())
		}
		fi := &plan.fields[i]
		target := pv.Field(fi.index).Addr().Interface()
		if err := fi.decodeInto(target, elem); err != nil {
			return Err(InvalidParams, err.Error())
		}
		i++
	}
	if i < plan.requiredN {
		return Err(InvalidParams, "missing/unexpected params in list")
	}
	if v != nil && plan.needsValidation {
		if err := v.Struct(p); err != nil {
			return Err(InvalidParams, err.Error())
		}
	}
	return nil
}

// decodeObject unmarshals once into map[name]RawMessage, validates
// missing-required and unknown-key invariants, then decodes each
// present field directly from its map entry into the matching slot in
// *p. No secondary whole-struct unmarshal.
func decodeObject[P any](raw json.RawMessage, plan *regPlan, p *P, v Validator) *Error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return Err(InvalidParams, err.Error())
	}
	for i := range plan.fields {
		fi := &plan.fields[i]
		if fi.optional {
			continue
		}
		if _, ok := m[fi.name]; !ok {
			return Err(InvalidParams, "missing non-optional param: "+fi.name)
		}
	}
	var unknown []string
	for key := range m {
		if _, ok := plan.byName[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		if len(unknown) > 1 {
			sortBySourceOrder(unknown, raw)
		}
		return Err(InvalidParams, "unexpected params: "+strings.Join(unknown, ", "))
	}
	pv := reflect.ValueOf(p).Elem()
	for i := range plan.fields {
		fi := &plan.fields[i]
		elem, ok := m[fi.name]
		if !ok {
			continue
		}
		target := pv.Field(fi.index).Addr().Interface()
		if err := fi.decodeInto(target, elem); err != nil {
			return Err(InvalidParams, err.Error())
		}
	}
	if v != nil && plan.needsValidation {
		if err := v.Struct(p); err != nil {
			return Err(InvalidParams, err.Error())
		}
	}
	return nil
}

// sortBySourceOrder reorders names by the position of their quoted
// occurrence in raw. Used only on the unknown-key error path so the
// emitted list matches the order the client sent.
func sortBySourceOrder(names []string, raw []byte) {
	slices.SortFunc(names, func(a, b string) int {
		return bytes.Index(raw, []byte(`"`+a+`"`)) - bytes.Index(raw, []byte(`"`+b+`"`))
	})
}

// Public entry points. P is the param struct (use NoParams for none).
// Four variants by ctx × header — API ergonomics so handlers stay
// terse on the axes they don't use.
//
//   Register    — func(*P) (R, *Error)
//   RegisterC   — func(ctx, *P) (R, *Error)
//   RegisterH   — func(*P) (R, http.Header, *Error)
//   RegisterCH  — func(ctx, *P) (R, http.Header, *Error)

func Register[P, R any](name string, fn func(*P) (R, *Error)) Method {
	plan := captureFieldTags[P]()
	return methodFrom[P, R](name,
		func(_ context.Context, v Validator, raw json.RawMessage) (any, http.Header, *Error) {
			var p P
			if e := decodeParams(raw, plan, &p, v); e != nil {
				return nil, nil, e
			}
			r, e := fn(&p)
			return r, nil, e
		})
}

func RegisterC[P, R any](name string, fn func(context.Context, *P) (R, *Error)) Method {
	plan := captureFieldTags[P]()
	return methodFrom[P, R](name,
		func(ctx context.Context, v Validator, raw json.RawMessage) (any, http.Header, *Error) {
			var p P
			if e := decodeParams(raw, plan, &p, v); e != nil {
				return nil, nil, e
			}
			r, e := fn(ctx, &p)
			return r, nil, e
		})
}

func RegisterH[P, R any](name string, fn func(*P) (R, http.Header, *Error)) Method {
	plan := captureFieldTags[P]()
	return methodFrom[P, R](name,
		func(_ context.Context, v Validator, raw json.RawMessage) (any, http.Header, *Error) {
			var p P
			if e := decodeParams(raw, plan, &p, v); e != nil {
				return nil, nil, e
			}
			return fn(&p)
		})
}

func RegisterCH[P, R any](
	name string, fn func(context.Context, *P) (R, http.Header, *Error),
) Method {
	plan := captureFieldTags[P]()
	return methodFrom[P, R](name,
		func(ctx context.Context, v Validator, raw json.RawMessage) (any, http.Header, *Error) {
			var p P
			if e := decodeParams(raw, plan, &p, v); e != nil {
				return nil, nil, e
			}
			return fn(ctx, &p)
		})
}

func methodFrom[P, R any](name string, d Dispatch) Method {
	return Method{
		Name:            name,
		Dispatch:        d,
		ParamStructType: reflect.TypeFor[P](),
	}
}

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

// unmarshalParams decodes b into p with json.Number semantics so numeric
// literals don't lose precision when round-tripping through RawMessage.
func unmarshalParams(b []byte, p any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return dec.Decode(p)
}

// regPlan is the per-method metadata captured at registration. Hot
// path reads it as plain slice / map lookups.
type regPlan struct {
	tags      []string       // json tag name in declaration order
	optional  []bool         // matches tags[i]
	byName    map[string]int // tag name → index
	requiredN int
}

// captureFieldTags walks P once at registration. P must be a struct;
// every exported field must carry a `json:"name[,omitempty]"` tag.
// `omitempty` flags the field as optional. Reflection only happens here
func captureFieldTags[P any]() *regPlan {
	pt := reflect.TypeFor[P]()
	if pt.Kind() != reflect.Struct {
		panic(fmt.Sprintf("jsonrpc: param type %s must be a struct", pt))
	}
	plan := &regPlan{}
	if pt.NumField() == 0 {
		return plan
	}
	plan.tags = make([]string, 0, pt.NumField())
	plan.optional = make([]bool, 0, pt.NumField())
	plan.byName = make(map[string]int, pt.NumField())
	for f := range pt.Fields() {
		tag, ok := f.Tag.Lookup("json")
		if !ok {
			panic(fmt.Sprintf("jsonrpc: %s.%s missing `json:` tag", pt, f.Name))
		}
		name, optsRaw, _ := strings.Cut(tag, ",")
		// A param is optional if its json tag carries either
		// `omitempty` or `omitzero` as one of the comma-separated options.
		opts := strings.Split(optsRaw, ",")
		optional := slices.Contains(opts, "omitempty") || slices.Contains(opts, "omitzero")
		plan.byName[name] = len(plan.tags)
		plan.tags = append(plan.tags, name)
		plan.optional = append(plan.optional, optional)
		if !optional {
			plan.requiredN++
		}
	}
	return plan
}

// decodeParams routes raw JSON params into *p. Arity/missing/unknown
// detection produces error strings that match the JSON-RPC server's error format.
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

// guestimatedTagLen is an average tag len used across RPC params.
// Used to hint buffer size and reduce re-allocations.
const guestimatedTagLen = 16

// decodeArray converts a positional `[v1, v2, ...]` into the named form
// `{tag0: v1, tag1: v2, ...}` using the cached tag list, then defers
// the actual unmarshal to the json decoder.
func decodeArray[P any](raw json.RawMessage, plan *regPlan, p *P, v Validator) *Error {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return Err(InvalidParams, err.Error())
	}
	if len(arr) > len(plan.tags) || len(arr) < plan.requiredN {
		return Err(InvalidParams, "missing/unexpected params in list")
	}
	var buf bytes.Buffer
	buf.Grow(len(raw) + len(plan.tags)*guestimatedTagLen)
	buf.WriteByte('{')
	for i, elem := range arr {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('"')
		buf.WriteString(plan.tags[i])
		buf.WriteString(`":`)
		buf.Write(elem)
	}
	buf.WriteByte('}')
	if err := unmarshalParams(buf.Bytes(), p); err != nil {
		return Err(InvalidParams, err.Error())
	}
	if v != nil {
		if err := v.Struct(p); err != nil {
			return Err(InvalidParams, err.Error())
		}
	}
	return nil
}

// decodeObject validates a named-form params object then unmarshals it.
// One token walk to check missing-required / unknown-key invariants in
// source order, then unmarshalParams for the actual field assignment.
func decodeObject[P any](raw json.RawMessage, plan *regPlan, p *P, v Validator) *Error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return Err(InvalidParams, err.Error())
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return Err(InvalidParams, "params object expected")
	}
	found := make([]bool, len(plan.tags))
	var unknown []string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return Err(InvalidParams, err.Error())
		}
		key, ok := keyTok.(string)
		if !ok {
			return Err(InvalidParams, "non-string object key")
		}
		var elemRaw json.RawMessage
		if err := dec.Decode(&elemRaw); err != nil {
			return Err(InvalidParams, err.Error())
		}
		idx, ok := plan.byName[key]
		if !ok {
			unknown = append(unknown, key)
			continue
		}
		found[idx] = true
	}
	for i, name := range plan.tags {
		if !found[i] && !plan.optional[i] {
			return Err(InvalidParams, "missing non-optional param: "+name)
		}
	}
	if len(unknown) > 0 {
		return Err(InvalidParams, "unexpected params: "+strings.Join(unknown, ", "))
	}
	if err := unmarshalParams(raw, p); err != nil {
		return Err(InvalidParams, err.Error())
	}
	if v != nil {
		if err := v.Struct(p); err != nil {
			return Err(InvalidParams, err.Error())
		}
	}
	return nil
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

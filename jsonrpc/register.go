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

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
)

type NoParams = struct{}

var paramDecoder = sonic.Config{
	UseNumber: true,
}.Froze()

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

// decodeArray converts a positional `[v1, v2, ...]` into the named form
// `{tag0: v1, tag1: v2, ...}` using the cached tag list, then defers
// the actual unmarshal to sonic.
func decodeArray[P any](raw json.RawMessage, plan *regPlan, p *P, v Validator) *Error {
	var node ast.Node
	if err := node.UnmarshalJSON(raw); err != nil {
		return Err(InvalidParams, err.Error())
	}
	iter, err := node.Values()
	if err != nil {
		return Err(InvalidParams, err.Error())
	}
	var buf bytes.Buffer
	buf.Grow(len(raw) + len(plan.tags)*16)
	buf.WriteByte('{')
	i := 0
	var elem ast.Node
	for iter.Next(&elem) {
		if i >= len(plan.tags) {
			return Err(InvalidParams, "missing/unexpected params in list")
		}
		elemRaw, rawErr := elem.Raw()
		if rawErr != nil {
			return Err(InvalidParams, rawErr.Error())
		}
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('"')
		buf.WriteString(plan.tags[i])
		buf.WriteString(`":`)
		buf.WriteString(elemRaw)
		i++
	}
	if i < plan.requiredN {
		return Err(InvalidParams, "missing/unexpected params in list")
	}
	buf.WriteByte('}')
	if err := paramDecoder.Unmarshal(buf.Bytes(), p); err != nil {
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
// Two ast.Node walks: one to check missing-required / unknown-key
// invariants, then sonic.Unmarshal for the actual field assignment.
func decodeObject[P any](raw json.RawMessage, plan *regPlan, p *P, v Validator) *Error {
	var node ast.Node
	if err := node.UnmarshalJSON(raw); err != nil {
		return Err(InvalidParams, err.Error())
	}
	iter, err := node.Properties()
	if err != nil {
		return Err(InvalidParams, err.Error())
	}
	found := make([]bool, len(plan.tags))
	var unknown []string
	var pair ast.Pair
	for iter.Next(&pair) {
		idx, ok := plan.byName[pair.Key]
		if !ok {
			unknown = append(unknown, pair.Key)
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
	if err := paramDecoder.Unmarshal(raw, p); err != nil {
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
		pretouchTypes:   []reflect.Type{reflect.TypeFor[P](), reflect.TypeFor[R]()},
	}
}

package rpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/goccy/go-json"
	"github.com/iancoleman/strcase"
	"io"
	"net/http"
	"reflect"
	"runtime"
)

var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
)

// Handler links a method of JSON-RPC request.
type Handler interface {
	ServeJSONRPC(c context.Context, params *json.RawMessage) (result interface{}, err *Error)
}

// HandlerFunc type is an adapter to allow the use of
// ordinary functions as JSONRPC handlers. If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// jsonrpc.Handler that calls f.
type HandlerFunc func(c context.Context, params *json.RawMessage) (result interface{}, err *Error)

// ServeJSONRPC calls f(w, r).
func (f HandlerFunc) ServeJSONRPC(c context.Context, params *json.RawMessage) (result interface{}, err *Error) {
	return f(c, params)
}

// ServeHTTP provides basic JSON-RPC handling.
func (mr *MethodRepository) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rs, batch, err := ParseRequest(r)
	if err != nil {
		err := SendResponse(w, []*Response{
			{
				Version: Version,
				Error:   err,
			},
		}, false)
		if err != nil {
			fmt.Fprint(w, "Failed to encode error objects")
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	resp := make([]*Response, len(rs))
	for i := range rs {
		resp[i] = mr.InvokeMethod(r.Context(), rs[i])
	}

	if err := SendResponse(w, resp, batch); err != nil {
		fmt.Fprint(w, "Failed to encode result objects")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// Does t satisfy the error interface?
func isErrorType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Implements(errorType)
}

func checkReturn(fntype reflect.Type) int {
	// Verify return types. The function must return at most one error
	// and/or one other non-error value.
	outs := make([]reflect.Type, fntype.NumOut())
	for i := 0; i < fntype.NumOut(); i++ {
		outs[i] = fntype.Out(i)
	}
	if len(outs) > 2 {
		return -1
	}
	// If an error is returned, it must be the last returned value.
	switch {
	case len(outs) == 1 && isErrorType(outs[0]):
		return 1
	case len(outs) == 2:
		if isErrorType(outs[0]) || !isErrorType(outs[1]) {
			return -1
		}
		return 1
	}
	return 0
}

// makeArgTypes composes the argTypes list.
func makeArgTypes(fn, rcvr reflect.Value) ([]reflect.Type, bool) {
	functionType := fn.Type()
	hasCtx := false

	// Skip receiver and context.Context parameter (if present).
	firstArg := 0
	if rcvr.IsValid() {
		firstArg++
	}
	if functionType.NumIn() > firstArg && functionType.In(firstArg) == contextType {
		hasCtx = true
		firstArg++
	}
	// Add all remaining parameters.
	argTypes := make([]reflect.Type, functionType.NumIn()-firstArg)
	for i := firstArg; i < functionType.NumIn(); i++ {
		argTypes[i-firstArg] = functionType.In(i)
	}
	return argTypes, hasCtx
}

// parsePositionalArguments tries to parse the given args to an array of values with the
// given types. It returns the parsed values or an error when the args could not be
// parsed. Missing optional arguments are returned as reflect.Zero values.
func parsePositionalArguments(rawArgs json.RawMessage, types []reflect.Type) ([]reflect.Value, error) {
	dec := json.NewDecoder(bytes.NewReader(rawArgs))
	var args []reflect.Value
	tok, err := dec.Token()
	switch {
	case err == io.EOF || tok == nil && err == nil:
		// "params" is optional and may be empty. Also allow "params":null even though it's
		// not in the spec because our own client used to send it.
	case err != nil:
		return nil, err
	case tok == json.Delim('['):
		// Read argument array.
		if args, err = parseArgumentArray(dec, types); err != nil {
			return nil, err
		}
	case tok == json.Delim('{'):
		argval := reflect.New(types[0])
		if err := json.Unmarshal(rawArgs, argval.Interface()); err != nil {
			return args, fmt.Errorf("invalid argument %d: %v", 0, err)
		}
		if argval.IsNil() && types[0].Kind() != reflect.Ptr {
			return args, fmt.Errorf("missing value for required argument %d", 0)
		}
		args = append(args, argval.Elem())
	default:
		return nil, errors.New("non-array args")
	}
	// Set any missing args to nil.
	for i := len(args); i < len(types); i++ {
		if types[i].Kind() != reflect.Ptr {
			return nil, fmt.Errorf("missing value for required argument %d", i)
		}
		args = append(args, reflect.Zero(types[i]))
	}
	return args, nil
}

func parseArgumentArray(dec *json.Decoder, types []reflect.Type) ([]reflect.Value, error) {
	args := make([]reflect.Value, 0, len(types))
	for i := 0; dec.More(); i++ {
		if i >= len(types) {
			return args, fmt.Errorf("too many arguments, want at most %d", len(types))
		}
		argval := reflect.New(types[i])
		if err := dec.Decode(argval.Interface()); err != nil {
			return args, fmt.Errorf("invalid argument %d: %v", i, err)
		}
		if argval.IsNil() && types[i].Kind() != reflect.Ptr {
			return args, fmt.Errorf("missing value for required argument %d", i)
		}
		args = append(args, argval.Elem())
	}
	// Read end of args array.
	_, err := dec.Token()
	return args, err
}

// call invokes the callback.
func call(ctx context.Context, method string, args []reflect.Value, fn, rcvr reflect.Value, hasCtx bool, errPos int) (res interface{}, errRes error) {
	// Create the argument slice.
	fullargs := make([]reflect.Value, 0, 2+len(args))
	if rcvr.IsValid() {
		fullargs = append(fullargs, rcvr)
	}
	if hasCtx {
		fullargs = append(fullargs, reflect.ValueOf(ctx))
	}
	fullargs = append(fullargs, args...)

	// Catch panic while running the callback.
	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			fmt.Printf("RPC method " + method + " crashed: " + fmt.Sprintf("%v\n%s", err, buf))
			errRes = errors.New("method handler crashed")
		}
	}()
	// Run the callback.
	results := fn.Call(fullargs)
	if len(results) == 0 {
		return nil, nil
	}
	if errPos >= 0 && !results[errPos].IsNil() {
		// Method has returned non-nil error value.
		err := results[errPos].Interface().(error)
		return reflect.Value{}, err
	}
	return results[0].Interface(), nil
}

// InvokeMethod invokes JSON-RPC method.
func (mr *MethodRepository) InvokeMethod(c context.Context, r *Request) *Response {
	res := NewResponse(r)
	structToCall := reflect.ValueOf(mr.MethodsToCall)

	function, ok := structToCall.Type().MethodByName(strcase.ToCamel(r.Method))
	if !ok {
		fmt.Printf("Method %s dosn't exist\n", r.Method)
		res.Result = nil
		res.Error = ErrMethodNotFound()
		return res
	}
	fmt.Printf("Method %s exist\n", r.Method)

	argsType, hasCtx := makeArgTypes(function.Func, structToCall)

	args, _ := parsePositionalArguments(*r.Params, argsType)

	errPos := checkReturn(function.Type)

	resFromCall, err := call(c, r.Method, args, function.Func, structToCall, hasCtx, errPos)

	if err != nil {
		res.Error = ErrInternal()
		res.Result = nil
	}
	res.Result = resFromCall
	return res
}

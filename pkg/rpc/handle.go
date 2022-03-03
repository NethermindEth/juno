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
	"strings"
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

// checkReturn Verify return types. The function must return at most one error
//	and/or one other non-error value.
func checkReturn(functionType reflect.Type) int {
	// Create a representation for each type of the Outputs of the function
	outs := make([]reflect.Type, functionType.NumOut())
	for i := 0; i < functionType.NumOut(); i++ {
		outs[i] = functionType.Out(i)
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

// makeArgumentTypes composes the argTypes list.
func makeArgumentTypes(function, receiver reflect.Value) ([]reflect.Type, bool) {
	functionType := function.Type()
	hasContext := false

	// Skip receiver and context.Context parameter (if present).
	firstArg := 0
	if receiver.IsValid() {
		firstArg++
	}
	if functionType.NumIn() > firstArg && functionType.In(firstArg) == contextType {
		hasContext = true
		firstArg++
	}
	// Add all remaining parameters.
	argumentTypes := make([]reflect.Type, functionType.NumIn()-firstArg)
	for i := firstArg; i < functionType.NumIn(); i++ {
		argumentTypes[i-firstArg] = functionType.In(i)
	}
	return argumentTypes, hasContext
}

// parseArguments tries to parse the given arguments to an array of values with the
// given types or the corresponding type. It returns the parsed values or an error when the args could not be
// parsed. Missing optional arguments are returned as reflect.Zero values.
func parseArguments(rawMessage json.RawMessage, types []reflect.Type) ([]reflect.Value, error) {
	decoder := json.NewDecoder(bytes.NewReader(rawMessage))
	var arguments []reflect.Value
	token, err := decoder.Token()
	switch {
	case err == io.EOF || token == nil && err == nil:
		// "params" is optional and may be empty. Also allow "params":null even though it's
		// not in the spec because our own client used to send it.
	case err != nil:
		return nil, err
	case token == json.Delim('['):
		// Read argument array.
		if arguments, err = parseArgumentArray(decoder, types); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("non-array or struct arguments")
	}
	// Set any missing arguments to nil.
	for i := len(arguments); i < len(types); i++ {
		if types[i].Kind() != reflect.Ptr {
			return nil, fmt.Errorf("missing value for required argument %d", i)
		}
		arguments = append(arguments, reflect.Zero(types[i]))
	}
	return arguments, nil
}

// parseArgumentArray Iterate overs each types and tries to parse the given arguments to an array of values
func parseArgumentArray(dec *json.Decoder, types []reflect.Type) ([]reflect.Value, error) {
	arguments := make([]reflect.Value, 0, len(types))
	for i := 0; dec.More(); i++ {
		if i >= len(types) {
			return arguments, fmt.Errorf("too many arguments, want at most %d", len(types))
		}
		argumentValue := reflect.New(types[i])
		if err := dec.Decode(argumentValue.Interface()); err != nil {
			return arguments, fmt.Errorf("invalid argument %d: %v", i, err)
		}
		if argumentValue.IsNil() && types[i].Kind() != reflect.Ptr {
			return arguments, fmt.Errorf("missing value for required argument %d", i)
		}
		logger.With("Kind", argumentValue.Elem().Kind()).Info("Checking kind")
		if argumentValue.Elem().Kind() == reflect.Struct {

			fields := argumentValue.Elem()
			logger.With("Number of fields", fields.NumField()).Debug("Parsing Parameters")
			for i := 0; i < fields.NumField(); i++ {
				t := fields.Type().Name()
				requiredTag := fields.Type().Field(i).Tag.Get("required")
				logger.With("Type", t, "Tag", requiredTag).Debug("Parsing Parameter")
				if strings.Contains(requiredTag, "true") && fields.Field(i).IsZero() {
					return arguments, errors.New("required field is missing")
				}

			}
		}
		arguments = append(arguments, argumentValue.Elem())
	}
	// Read end of arguments array.
	_, err := dec.Token()
	return arguments, err
}

// callFunction invokes the function called.
func callFunction(ctx context.Context, method string, arguments []reflect.Value, function, receiver reflect.Value,
	hasContext bool, errResponsePosition int) (res interface{}, errRes error) {
	logger.With("Method", method).Info("Calling RPC function")
	// Create the argument slice.
	fullArguments := make([]reflect.Value, 0, 2+len(arguments))
	if receiver.IsValid() {
		fullArguments = append(fullArguments, receiver)
	}
	if hasContext {
		fullArguments = append(fullArguments, reflect.ValueOf(ctx))
	}
	fullArguments = append(fullArguments, arguments...)

	// Catch panic while running the callback.
	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			logger.With(
				"Method", method,
				"Error", fmt.Sprintf("%v\n%s", err, buf),
				"Arguments", arguments,
				"Has Context", hasContext,
				"Error Response Position", errResponsePosition).
				Error("RPC method crashed")
			errRes = errors.New("method handler crashed")
		}
	}()
	// Run the function.
	results := function.Call(fullArguments)
	if len(results) == 0 {
		return nil, nil
	}
	if errResponsePosition >= 0 && !results[errResponsePosition].IsNil() {
		// Method has returned non-nil error value.
		err := results[errResponsePosition].Interface().(error)
		return reflect.Value{}, err
	}
	return results[0].Interface(), nil
}

// InvokeMethod invokes JSON-RPC method.
func (mr *MethodRepository) InvokeMethod(c context.Context, r *Request) *Response {
	res := NewResponse(r)
	structToCall := reflect.ValueOf(mr.StructRpc)

	// Check that the struct contains the method called
	function, ok := structToCall.Type().MethodByName(strcase.ToCamel(r.Method))
	if !ok {
		logger.With("Method", r.Method).Error("Method didn't exist")
		res.Result = nil
		res.Error = ErrMethodNotFound()
		return res
	}

	// Get all the types of the arguments of the function that is going to be called
	argumentTypes, hasContext := makeArgumentTypes(function.Func, structToCall)

	// Parse all the params received in the request and cast it to the types of the function that is going to be called
	args, err := parseArguments(*r.Params, argumentTypes)
	if err != nil {
		logger.With("Method", r.Method, "Params", r.Params).Error("Invalid params")
		res.Result = nil
		res.Error = ErrInvalidParams()
		return res
	}

	// Check the return of the function and get the error position (Methods should always return and error)
	errPos := checkReturn(function.Type)
	logger.With("Args:", args[0].Interface()).Info("Argument")
	// Call the function
	resFromCall, err := callFunction(c, r.Method, args, function.Func, structToCall, hasContext, errPos)
	if err != nil {
		logger.With("Method", r.Method,
			"Params", r.Params,
		).Error("Internal error calling the function")
		res.Error = ErrInternal()
		res.Result = nil
	}
	logger.With("Method", r.Method).Info("RPC handle request successfully")
	res.Result = resFromCall
	return res
}

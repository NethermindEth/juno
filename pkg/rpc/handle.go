package rpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime"
	"strings"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/goccy/go-json"
	"github.com/iancoleman/strcase"
)

var (
	contextType      = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType        = reflect.TypeOf((*error)(nil)).Elem()
	errorTooManyArgs = errors.New("too many arguments")
)

// Handler links a method of JSON-RPC request.
type Handler interface {
	ServeJSONRPC(
		c context.Context, params *json.RawMessage,
	) (result interface{}, err *Error)
}

// HandlerFunc type is an adapter that allows the use of Go functions as
// JSON-RPC handlers. If f is such a function with the appropriate
// signature, HandlerFunc(f) is a JSON-RPC.Handler that calls f.
type HandlerFunc func(
	c context.Context, params *json.RawMessage,
) (result interface{}, err *Error)

// ServeJSONRPC calls a function f(w, r).
func (f HandlerFunc) ServeJSONRPC(
	c context.Context, params *json.RawMessage,
) (result interface{}, err *Error) {
	// notest
	return f(c, params)
}

// ServeHTTP provides basic JSON-RPC handling.
func (h *HandlerJsonRpc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rs, batch, err := ParseRequest(r)
	if err != nil {
		err := SendResponse(w, []*Response{{Version: Version, Error: err}}, false)
		if err != nil {
			// notest
			log.Default.With("Error", err).Info("Failed to encode error object.")
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	res := make([]*Response, len(rs))
	for i, req := range rs {
		res[i] = h.InvokeMethod(r.Context(), req)
	}

	if err := SendResponse(w, res, batch); err != nil {
		// notest
		log.Default.With("Error", err).Info("Failed to encode result object.")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// isErrorType checks whether t implements the error interface.
func isErrorType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		// notest
		t = t.Elem()
	}
	return t.Implements(errorType)
}

// checkReturn verifies the return types. The function must return at
// most one error and/or one other non-error value.
func checkReturn(fnType reflect.Type) int {
	// Create a representation for each type of the Outputs of the
	// function.
	outs := make([]reflect.Type, fnType.NumOut())
	for i := 0; i < fnType.NumOut(); i++ {
		outs[i] = fnType.Out(i)
	}
	if len(outs) > 2 {
		// notest
		return -1
	}
	// If an error is returned, it must be the last returned value.
	switch {
	case len(outs) == 1 && isErrorType(outs[0]):
		// notest
		return 1
	case len(outs) == 2:
		if isErrorType(outs[0]) || !isErrorType(outs[1]) {
			// notest
			return -1
		}
		return 1
	}
	// notest
	return 0
}

// makeArgTypes composes the argTypes list.
func makeArgTypes(fn, receiver reflect.Value) ([]reflect.Type, bool) {
	funcType := fn.Type()
	hasContext := false

	// Skip receiver and context.Context parameter (if present).
	firstArg := 0
	if receiver.IsValid() {
		firstArg++
	}
	if funcType.NumIn() > firstArg && funcType.In(firstArg) == contextType {
		hasContext = true
		firstArg++
	}
	// Add all remaining parameters.
	argTypes := make([]reflect.Type, funcType.NumIn()-firstArg)
	for i := firstArg; i < funcType.NumIn(); i++ {
		argTypes[i-firstArg] = funcType.In(i)
	}
	return argTypes, hasContext
}

// parseArgs tries to parse the given arguments into an array of values
// with the given types or the corresponding type. It returns the parsed
// values or an error otherwise. Missing optional arguments are returned
// as reflect.Zero values.
func parseArgs(
	rawMessage json.RawMessage, types []reflect.Type,
) ([]reflect.Value, error) {
	decoder := json.NewDecoder(bytes.NewReader(rawMessage))
	var args []reflect.Value
	token, err := decoder.Token()
	switch {
	// notest
	case err == io.EOF || token == nil && err == nil:
		// "params" is optional and may be empty. Also allow "params":null
		// even though it's not in the spec because our own client used to
		// send it.
	case err != nil:
		return nil, err
	case token == json.Delim('['):
		// Read argument array.
		if args, err = parseArgSlice(decoder, types); err != nil {
			return nil, err
		}
	default:
		// notest
		return nil, errors.New("non-array or struct arguments")
	}
	// Set any missing arguments to nil.
	for i := len(args); i < len(types); i++ {
		// notest
		if types[i].Kind() != reflect.Ptr {
			return nil, fmt.Errorf("missing value for required argument %d", i)
		}
		args = append(args, reflect.Zero(types[i]))
	}
	return args, nil
}

// parseArgSlice iterate overs each types and tries to parse the given
// arguments to an array of values.
func parseArgSlice(
	decoder *json.Decoder, types []reflect.Type,
) ([]reflect.Value, error) {
	decoder.UseNumber()

	args := make([]reflect.Value, 0, len(types))
	for i := 0; decoder.More(); i++ {
		if i >= len(types) {
			err := errorTooManyArgs
			log.Default.With("Error", err, "Requires at most ", len(types))
			return args, err
		}
		val := reflect.New(types[i])
		if err := decoder.Decode(val.Interface()); err != nil {
			//if val.Elem().Kind() == reflect.Int64 && errType.Value == "number" {
			//	var msgUser msgUserIntVal
			//	err = json.Unmarshal(payload, &msgUser)
			//	if nil == err {
			//		msgUser.User.StrValue = strconv.Itoa(msgUser.StrValue)
			//		user = *msgUser.User
			//	}
			//}
			return args, fmt.Errorf("invalid argument %d: %v", i, err)
		}
		if val.IsNil() && types[i].Kind() != reflect.Ptr {
			// notest
			return args, fmt.Errorf("missing value for required argument %d", i)
		}
		log.Default.With("Kind", val.Elem().Kind()).Info("Checking kind.")
		if val.Elem().Kind() == reflect.Struct {
			fields := val.Elem()
			log.Default.With("Number of fields", fields.NumField()).Debug("Parsing parameters.")
			for i := 0; i < fields.NumField(); i++ {
				t := fields.Type().Name()
				tag := fields.Type().Field(i).Tag.Get("required")
				log.Default.With("Type", t, "Tag", tag).Debug("Parsing parameter.")
				if strings.Contains(tag, "true") && fields.Field(i).IsZero() {
					// notest
					// XXX: Highlight which missing field this is.
					return args, errors.New("missing required field")
				}
			}
		}
		args = append(args, val.Elem())
	}
	// Read end of arguments array.
	_, err := decoder.Token()
	return args, err
}

// callFunc invokes the function called.
func callFunc(
	ctx context.Context,
	method string,
	args []reflect.Value,
	fn,
	receiver reflect.Value,
	hasContext bool,
	errResponsePosition int,
) (res interface{}, errRes error) {
	log.Default.With("Method", method).Info("Calling RPC function.")
	// Create the argument slice.
	fullArgs := make([]reflect.Value, 0, 2+len(args))
	if receiver.IsValid() {
		fullArgs = append(fullArgs, receiver)
	}
	if hasContext {
		fullArgs = append(fullArgs, reflect.ValueOf(ctx))
	}
	fullArgs = append(fullArgs, args...)

	// Catch panic while running the callback.
	defer func() {
		// notest
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			log.Default.With(
				"Method", method,
				"Error", fmt.Sprintf("%v\n%s", err, buf),
				"Arguments", args,
				"Has Context", hasContext,
				"Error Response Position", errResponsePosition,
			).Error("RPC method crashed.")
			errRes = errors.New("method handler crashed")
		}
	}()
	// Run the function.
	results := fn.Call(fullArgs)
	if len(results) == 0 {
		// notest
		return nil, nil
	}
	if errResponsePosition >= 0 && !results[errResponsePosition].IsNil() {
		// notest
		// Method has returned non-nil error value.
		err := results[errResponsePosition].Interface().(error)
		return reflect.Value{}, err
	}
	return results[0].Interface(), nil
}

// InvokeMethod invokes JSON-RPC method.
func (h *HandlerJsonRpc) InvokeMethod(
	c context.Context, r *Request,
) *Response {
	res := NewResponse(r)
	structToCall := reflect.ValueOf(h.StructRpc)

	// Check that the struct contains the method called
	fn, ok := structToCall.Type().MethodByName(strcase.ToCamel(r.Method))
	if !ok {
		// notest
		log.Default.With("Method", r.Method).Error("Method does not exist.")
		res.Result = nil
		res.Error = ErrMethodNotFound()
		return res
	}

	// Get all the types of the arguments of the function that is going to
	// be called.
	argTypes, hasContext := makeArgTypes(fn.Func, structToCall)
	var args []reflect.Value
	var err error
	if r.Params != nil {
		// Parse all the params received in the request and cast it to the
		// types of the function that is going to be called.
		args, err = parseArgs(*r.Params, argTypes)
		if err != nil {
			// XXX: Use more robust error handling instead of just matching
			// against a substring.
			if err == errorTooManyArgs {
				log.Default.Info("Searching for overload...")
				structToCall = reflect.ValueOf(h.StructRpc)

				// Check that the struct contains the method called.
				fn, ok = structToCall.Type().MethodByName(
					strcase.ToCamel(r.Method) + "Opt")
				if !ok {
					// notest
					log.Default.With(
						"Method", r.Method,
						"Params", r.Params,
						"Error", err,
					).Error("Invalid params.")
					res.Result = nil
					res.Error = ErrInvalidParams()
					return res
				}
				argTypes, hasContext = makeArgTypes(fn.Func, structToCall)

				args, err = parseArgs(*r.Params, argTypes)
				if err != nil {
					// notest
					log.Default.With(
						"Method", r.Method,
						"Params", r.Params,
						"Error", err,
					).Error("Invalid params")
					res.Result = nil
					res.Error = ErrInvalidParams()
					return res
				}
				log.Default.Info("Contains optional params, calling Overhead...")
			} else {
				// notest
				log.Default.With(
					"Method", r.Method,
					"Params", r.Params,
					"Error", err,
				).Error("Invalid params.")
				res.Result = nil
				res.Error = ErrInvalidParams()
				return res
			}
		}
	}
	// Check the return of the function and get the error position
	// (Methods should always return and report an error).
	errPos := checkReturn(fn.Type)
	// Call the function
	resFromCall, err := callFunc(
		c, r.Method, args, fn.Func, structToCall, hasContext, errPos)
	if err != nil {
		// notest
		log.Default.With(
			"Method", r.Method, "Params", r.Params,
		).Error("Internal error occurred while calling the function.")
		res.Error = ErrInternal()
		res.Result = nil
	}
	log.Default.With("Method", r.Method).Info("Request successful.")
	res.Result = resFromCall
	return res
}

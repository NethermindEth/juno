// Package jsonrpc implements a JSONRPC2.0 compliant server as described in https://www.jsonrpc.org/specification
package jsonrpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/NethermindEth/juno/metrics"
	"github.com/NethermindEth/juno/utils"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	InvalidJSON    = -32700 // Invalid JSON was received by the server.
	InvalidRequest = -32600 // The JSON sent is not a valid Request object.
	MethodNotFound = -32601 // The method does not exist / is not available.
	InvalidParams  = -32602 // Invalid method parameter(s).
	InternalError  = -32603 // Internal JSON-RPC error.
)

var ErrInvalidID = errors.New("id should be a string or an integer")

type request struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      any    `json:"id,omitempty"`
}

type response struct {
	Version string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
	ID      any    `json:"id"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func Err(code int, data any) *Error {
	switch code {
	case InvalidJSON:
		return &Error{Code: InvalidJSON, Message: "Parse error", Data: data}
	case InvalidRequest:
		return &Error{Code: InvalidRequest, Message: "Invalid Request", Data: data}
	case MethodNotFound:
		return &Error{Code: MethodNotFound, Message: "Method Not Found", Data: data}
	case InvalidParams:
		return &Error{Code: InvalidParams, Message: "Invalid Params", Data: data}
	default:
		return &Error{Code: InternalError, Message: "Internal Error", Data: data}
	}
}

func (r *request) isSane() error {
	if r.Version != "2.0" {
		return errors.New("unsupported RPC request version")
	}
	if r.Method == "" {
		return errors.New("no method specified")
	}

	if r.Params != nil {
		paramType := reflect.TypeOf(r.Params)
		if paramType.Kind() != reflect.Slice && paramType.Kind() != reflect.Map {
			return errors.New("params should be an array or an object")
		}
	}

	if r.ID != nil {
		idType := reflect.TypeOf(r.ID)
		floating := idType.Name() == "Number" && strings.Contains(r.ID.(json.Number).String(), ".")
		if (idType.Kind() != reflect.String && idType.Name() != "Number") || floating {
			return ErrInvalidID
		}
	}

	return nil
}

type Parameter struct {
	Name     string
	Optional bool
}

type Method struct {
	Name    string
	Params  []Parameter
	Handler any
}

type Server struct {
	callbacks   map[string]callback
	validator Validator
	log       utils.SimpleLogger

	// metrics
	requests *prometheus.CounterVec
}

type Validator interface {
	Struct(any) error
}

// NewServer instantiates a JSONRPC server
func NewServer(log utils.SimpleLogger) *Server {
	s := &Server{
		log:     log,
		callbacks: make(map[string]callback),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "rpc",
			Subsystem: "server",
			Name:      "requests",
		}, []string{"method"}),
	}

	metrics.MustRegister(s.requests)
	return s
}

// WithValidator registers a validator to validate handler struct arguments
func (s *Server) WithValidator(validator Validator) *Server {
	s.validator = validator
	return s
}

// RegisterMethod verifies and creates an endpoint that the server recognises.
//
// - name is the method name
// - handler is the function to be called when a request is received for the
// associated method. It should have (any, *jsonrpc.Error) as its return type
// - paramNames are the names of parameters in the order that they are expected
// by the handler
func (s *Server) RegisterMethod(method Method) error {
	callback, err := newCallback(method)
	if err != nil {
		return err
	}
	callback.parser = s.parseParam
	s.callbacks[method.Name] = callback
	return nil
}

// Handle processes a request to the server
// It returns the response in a byte array, only returns an
// error if it can not create the response byte array
func (s *Server) Handle(data []byte) ([]byte, error) {
	return s.HandleReader(bytes.NewReader(data))
}

// HandleReader processes a request to the server
// It returns the response in a byte array, only returns an
// error if it can not create the response byte array
func (s *Server) HandleReader(reader io.Reader) ([]byte, error) {
	bufferedReader := bufio.NewReader(reader)
	requestIsBatch := isBatch(bufferedReader)
	res := &response{
		Version: "2.0",
	}

	dec := json.NewDecoder(bufferedReader)
	dec.UseNumber()

	if !requestIsBatch {
		req := new(request)
		if jsonErr := dec.Decode(req); jsonErr != nil {
			res.Error = Err(InvalidJSON, jsonErr.Error())
		} else if resObject, handleErr := s.handleRequest(req); handleErr != nil {
			if !errors.Is(handleErr, ErrInvalidID) {
				res.ID = req.ID
			}
			res.Error = Err(InvalidRequest, handleErr.Error())
		} else {
			res = resObject
		}
	} else {
		var batchReq []json.RawMessage
		var batchRes []json.RawMessage

		if batchJSONErr := dec.Decode(&batchReq); batchJSONErr != nil {
			res.Error = Err(InvalidJSON, batchJSONErr.Error())
		} else if len(batchReq) == 0 {
			res.Error = Err(InvalidRequest, "empty batch")
		} else {
			for _, rawReq := range batchReq { // todo: handle async
				var resObject *response

				reqDec := json.NewDecoder(bytes.NewBuffer(rawReq))
				reqDec.UseNumber()

				req := new(request)
				if jsonErr := reqDec.Decode(req); jsonErr != nil {
					resObject = &response{
						Version: "2.0",
						Error:   Err(InvalidRequest, jsonErr.Error()),
					}
				} else {
					var handleErr error
					resObject, handleErr = s.handleRequest(req)
					if handleErr != nil {
						resObject = &response{
							Version: "2.0",
							Error:   Err(InvalidRequest, handleErr.Error()),
						}
						if !errors.Is(handleErr, ErrInvalidID) {
							resObject.ID = req.ID
						}
					}
				}

				if resObject != nil {
					if resArr, jsonErr := json.Marshal(resObject); jsonErr != nil {
						return nil, jsonErr
					} else {
						batchRes = append(batchRes, resArr)
					}
				}
			}

			if len(batchRes) == 0 {
				return nil, nil
			}
			return json.Marshal(batchRes)
		}
	}

	if res == nil {
		return nil, nil
	}
	return json.Marshal(res)
}

func isBatch(reader *bufio.Reader) bool {
	for {
		char, err := reader.Peek(1)
		if err != nil {
			break
		}
		if char[0] == ' ' || char[0] == '\t' || char[0] == '\r' || char[0] == '\n' {
			if discarded, err := reader.Discard(1); discarded != 1 || err != nil {
				break
			}
			continue
		}
		return char[0] == '['
	}
	return false
}

func isNil(i any) bool {
	return i == nil || reflect.ValueOf(i).IsNil()
}

func (s *Server) handleRequest(req *request) (*response, error) {
	start := time.Now()
	reqJSON, err := json.Marshal(req)
	if err == nil {
		s.log.Debugw("Serving RPC request", "request", string(reqJSON))
	}
	defer func() {
		s.log.Debugw("Responding to RPC request", "method", req.Method, "id", req.ID, "took", time.Since(start))
	}()

	if err = req.isSane(); err != nil {
		return nil, err
	}

	res := &response{
		Version: "2.0",
		ID:      req.ID,
	}

	callback, found := s.callbacks[req.Method]
	if !found {
		res.Error = Err(MethodNotFound, nil)
		return res, nil
	}

	args, err := callback.buildArgs(req.Params)
	if err != nil {
		res.Error = Err(InvalidParams, err.Error())
		return res, nil
	}

	s.requests.WithLabelValues(req.Method).Inc()
	notification := res.ID == nil
	result, resultError := callback.call(notification,args)
	if notification {
		return nil, nil
	}
	res.Result, res.Error = result, resultError
	return res, nil
}

func (s *Server) parseParam(param any, t reflect.Type) (reflect.Value, error) {
	handlerParam := reflect.New(t)
	valueMarshaled, err := json.Marshal(param) // we have to marshal the value into JSON again
	if err != nil {
		return reflect.ValueOf(nil), err
	}
	err = json.Unmarshal(valueMarshaled, handlerParam.Interface())
	if err != nil {
		return reflect.ValueOf(nil), err
	}

	elem := handlerParam.Elem()
	if s.validator != nil {
		if err = s.validateParam(elem); err != nil {
			return reflect.ValueOf(nil), err
		}
	}

	return elem, nil
}

func (s *Server) validateParam(param reflect.Value) error {
	kind := param.Kind()
	switch {
	case kind == reflect.Struct ||
		(kind == reflect.Pointer && param.Elem().Kind() == reflect.Struct):
		/* struct or a struct pointer */
		if err := s.validator.Struct(param.Interface()); err != nil {
			return err
		}
	case kind == reflect.Slice || kind == reflect.Array:
		for i := 0; i < param.Len(); i++ {
			if err := s.validateParam(param.Index(i)); err != nil {
				return err
			}
		}
	case kind == reflect.Map:
		for _, key := range param.MapKeys() {
			if err := s.validateParam(param.MapIndex(key)); err != nil {
				return err
			}
		}
	}

	return nil
}

type paramParser func(v any, t reflect.Type) (reflect.Value, error)

type callback struct {
	method Method
	returnsErr bool

	parser paramParser
}

func newCallback(method Method) (callback,error) { 
	var (
		callback = callback{method: method}
		handlerT = reflect.TypeOf(method.Handler)
	)
	if handlerT.Kind() != reflect.Func {
		return callback,errors.New("handler must be a function")
	}
	if handlerT.NumIn() != len(method.Params) {
		return callback,errors.New("number of function params and param names must match")
	}
	// Validate return values
	switch handlerT.NumOut(){
	case 0: // no return
	case 1: // can be either error or any
		callback.returnsErr = handlerT.Out(0) == reflect.TypeOf(&Error{})
	case 2: // second is always error
		callback.returnsErr = true
		if handlerT.Out(1) != reflect.TypeOf(&Error{}) {
			return callback, errors.New("second return value must be a *jsonrpc.Error")
		}
	default:
		return callback, errors.New("method return values can't be greater than 2")
	}
	return callback, nil
}

// Builds arguments for calling the underlying method
func (c callback) buildArgs(params any) ([]reflect.Value, error) { 
	args := make([]reflect.Value, 0, len(c.method.Params))
	if isNil(params) {
		if len(c.method.Params) > 0 {
			return nil, errors.New("missing non-optional param field")
		}

		return args, nil
	}

	handlerType := reflect.TypeOf(c.method.Handler)

	switch reflect.TypeOf(params).Kind() {
	case reflect.Slice:
		paramsList := params.([]any)

		if len(paramsList) != handlerType.NumIn() {
			return nil, errors.New("missing/unexpected params in list")
		}

		for i, param := range paramsList {
			v, err := c.parser(param, handlerType.In(i))
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
	case reflect.Map:
		paramsMap := params.(map[string]any)

		for i, configuredParam := range c.method.Params {
			var v reflect.Value
			if param, found := paramsMap[configuredParam.Name]; found {
				var err error
				v, err = c.parser(param, handlerType.In(i))
				if err != nil {
					return nil, err
				}
			} else if configuredParam.Optional {
				// optional parameter
				v = reflect.New(handlerType.In(i)).Elem()
			} else {
				return nil, errors.New("missing non-optional param")
			}

			args = append(args, v)
		}
	default:
		// Todo: consider returning InternalError
		return nil, errors.New("impossible param type: check request.isSane")
	}
	return args, nil
}

// Calls the underlying handler with provided args.
func (c callback) call(ignoreOutput bool,args []reflect.Value) (res any, errRes *Error) {
	tuple := reflect.ValueOf(c.method.Handler).Call(args)
	if ignoreOutput {
		return nil, nil
	}

	if c.returnsErr {
		if errAny := tuple[len(tuple)-1].Interface(); !isNil(errAny) {
			errRes = errAny.(*Error)
			return
		}
	}

	switch len(tuple) {
	case 2:
		res = tuple[0].Interface()
	case 1:
		if !c.returnsErr {
			res = tuple[0].Interface()
		}
	}
	return
}


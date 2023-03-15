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
)

// Todo: add rpcErr type which implements Error() to return short string representation of the error. For example:
//   - ErrInvalidJson.Error() = "Parse error"
//   - ErrInvalidRequest.Error() = "Invalid Request"
//   - ErrMethodNotFound.Error() = "Method Not Found"
//   - ErrInvalidParams.Error() = "Invalid Params"
//   - ErrInternal.Error() = "Internal Error"
//
// More contextual information can then be passed through Data field of Error as follows:
//
//	Error {
//	  Code: ErrInvalidRequest,
//	  Message: ErrInvalidRequest.Error(),
//	  Data: err.Error(), // These are golang errors to provide more information
//	}

const (
	InvalidJson    = -32700 // Invalid JSON was received by the server.
	InvalidRequest = -32600 // The JSON sent is not a valid Request object.
	MethodNotFound = -32601 // The method does not exist / is not available.
	InvalidParams  = -32602 // Invalid method parameter(s).
	InternalError  = -32603 // Internal JSON-RPC error.
)

var ErrInvalidId = errors.New("id should be a string or an integer")

type request struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	Id      any    `json:"id,omitempty"`
}

type response struct {
	Version string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
	Id      any    `json:"id"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (r *request) isSane() error {
	if r.Version != "2.0" {
		return errors.New("unsupported RPC request version")
	}
	if len(r.Method) <= 0 {
		return errors.New("no method specified")
	}

	if r.Params != nil {
		paramType := reflect.TypeOf(r.Params)
		if paramType.Kind() != reflect.Slice && paramType.Kind() != reflect.Map {
			return errors.New("params should be an array or an object")
		}
	}

	if r.Id != nil {
		idType := reflect.TypeOf(r.Id)
		floating := idType.Name() == "Number" && strings.Contains(r.Id.(json.Number).String(), ".")
		if (idType.Kind() != reflect.String && idType.Name() != "Number") || floating {
			return ErrInvalidId
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
	methods map[string]Method
}

// NewServer instantiates a JSONRPC server
func NewServer() *Server {
	return &Server{
		methods: make(map[string]Method),
	}
}

// RegisterMethod verifies and creates an endpoint that the server recognizes.
//
// - name is the method name
// - handler is the function to be called when a request is received for the
// associated method. It should have (any, *jsonrpc.Error) as its return type
// - paramNames are the names of parameters in the order that they are expected
// by the handler
func (s *Server) RegisterMethod(method Method) error {
	handlerT := reflect.TypeOf(method.Handler)
	if handlerT.Kind() != reflect.Func {
		return errors.New("handler must be a function")
	}
	if handlerT.NumIn() != len(method.Params) {
		return errors.New("number of function params and param names must match")
	}
	if handlerT.NumOut() != 2 {
		return errors.New("handler must return 2 values")
	}
	if handlerT.Out(1) != reflect.TypeOf(&Error{}) {
		return errors.New("second return value must be a *jsonrpc.Error")
	}

	s.methods[method.Name] = method

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
			res.Error = &Error{Code: InvalidJson, Message: jsonErr.Error()}
		} else if resObject, handleErr := s.handleRequest(req); handleErr != nil {
			if !errors.Is(handleErr, ErrInvalidId) {
				res.Id = req.Id
			}
			res.Error = &Error{Code: InvalidRequest, Message: handleErr.Error()}
		} else {
			res = resObject
		}
	} else {
		var batchReq []json.RawMessage
		var batchRes []json.RawMessage

		if batchJsonErr := dec.Decode(&batchReq); batchJsonErr != nil {
			res.Error = &Error{Code: InvalidJson, Message: batchJsonErr.Error()}
		} else if len(batchReq) == 0 {
			res.Error = &Error{Code: InvalidRequest, Message: "empty batch"}
		} else {
			for _, rawReq := range batchReq { // todo: handle async
				var resObject *response

				reqDec := json.NewDecoder(bytes.NewBuffer(rawReq))
				reqDec.UseNumber()

				req := new(request)
				if jsonErr := reqDec.Decode(req); jsonErr != nil {
					resObject = &response{
						Version: "2.0",
						Error:   &Error{Code: InvalidRequest, Message: jsonErr.Error()},
					}
				} else {
					var handleErr error
					resObject, handleErr = s.handleRequest(req)
					if handleErr != nil {
						resObject = &response{
							Version: "2.0",
							Error:   &Error{Code: InvalidRequest, Message: handleErr.Error()},
						}
						if !errors.Is(handleErr, ErrInvalidId) {
							resObject.Id = req.Id
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
		} else if char[0] == ' ' || char[0] == '\t' || char[0] == '\r' || char[0] == '\n' {
			if discarded, err := reader.Discard(1); discarded != 1 || err != nil {
				break
			}
			continue
		} else {
			return char[0] == '['
		}
	}

	return false
}

func isNil(i any) bool {
	return i == nil || reflect.ValueOf(i).IsNil()
}

func (s *Server) handleRequest(req *request) (*response, error) {
	if err := req.isSane(); err != nil {
		return nil, err
	}

	res := &response{
		Version: "2.0",
		Id:      req.Id,
	}

	calledMethod, found := s.methods[req.Method]
	if !found {
		res.Error = &Error{
			Code:    MethodNotFound,
			Message: "method not found",
		}
		return res, nil
	}

	args, err := buildArguments(req.Params, calledMethod.Handler, calledMethod.Params)
	if err != nil {
		res.Error = &Error{
			Code:    InvalidParams,
			Message: err.Error(),
		}
		return res, nil
	}

	tuple := reflect.ValueOf(calledMethod.Handler).Call(args)
	if res.Id == nil { // notification
		return nil, nil
	}

	if errAny := tuple[1].Interface(); !isNil(errAny) {
		res.Error = errAny.(*Error)
		return res, nil
	}

	res.Result = tuple[0].Interface()
	return res, nil
}

func buildArguments(params, handler any, configuredParams []Parameter) ([]reflect.Value, error) {
	var args []reflect.Value
	if isNil(params) {
		return args, nil
	}

	handlerType := reflect.TypeOf(handler)

	handlerParamValue := func(param any, t reflect.Type) (reflect.Value, error) {
		handlerParam := reflect.New(t)
		valueMarshaled, err := json.Marshal(param) // we have to marshal the value into JSON again
		if err != nil {
			return reflect.ValueOf(nil), err
		}
		err = json.Unmarshal(valueMarshaled, handlerParam.Interface())
		if err != nil {
			return reflect.ValueOf(nil), err
		}

		return handlerParam.Elem(), nil
	}

	switch reflect.TypeOf(params).Kind() {
	case reflect.Slice:
		paramsList := params.([]any)

		if len(paramsList) != handlerType.NumIn() {
			return nil, errors.New("missing/unexpected params in list")
		}

		for i, param := range paramsList {
			v, err := handlerParamValue(param, handlerType.In(i))
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
		return args, nil
	case reflect.Map:
		paramsMap := params.(map[string]any)

		for i, configuredParam := range configuredParams {
			var v reflect.Value
			if param, found := paramsMap[configuredParam.Name]; found {
				var err error
				v, err = handlerParamValue(param, handlerType.In(i))
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
		return args, nil
	default:
		// Todo: return InternalError
		return nil, errors.New("impossible param type: check request.isSane")
	}
}

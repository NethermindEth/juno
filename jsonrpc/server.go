package jsonrpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

const (
	InvalidJson    = -32700 // Invalid JSON was received by the server.
	InvalidRequest = -32600 // The JSON sent is not a valid Request object.
	MethodNotFound = -32601 // The method does not exist / is not available.
	InvalidParams  = -32602 // Invalid method parameter(s).
	InternalError  = -32603 // Internal JSON-RPC error.
)

type request struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
	Id      any    `json:"id"`
}

type batch []json.RawMessage

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

func newRequest(reader io.Reader) (*request, error) {
	req := new(request)
	dec := json.NewDecoder(reader)
	dec.UseNumber()
	if err := dec.Decode(req); err != nil {
		return nil, err
	} else if err = req.isSane(); err != nil {
		return nil, err
	}
	return req, nil
}

func (r *request) isSane() error {
	if r.Version != "2.0" {
		return errors.New(fmt.Sprintf("unsupported RPC request version"))
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
			return errors.New("id should be a string or an integer")
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

// RegisterMethod verifies and creates and endpoint that server recognizes.
//
// - name is the method name
// - handler is the function to be called when a request is received for the
// associated method. It should have (any, error) as it's return type
// - paramNames are the names of parameters in the order that they are expected
// by the handler
func (s *Server) RegisterMethod(name string, params []Parameter, handler any) error {
	handlerT := reflect.TypeOf(handler)
	if handlerT.Kind() != reflect.Func {
		return errors.New("handler must be a function")
	}
	if len(params) > 0 && handlerT.NumIn() != len(params) {
		return errors.New("number of function params and param names must match")
	}
	if handlerT.NumOut() != 2 {
		return errors.New("handler must return 2 values")
	}
	if handlerT.Out(1) != reflect.TypeOf((*Error)(nil)) {
		return errors.New("second return value must be a *jsonrpc.Error")
	}

	s.methods[name] = Method{
		Name:    name,
		Params:  params,
		Handler: handler,
	}

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

	if !isBatch(bufferedReader) {
		var res *response
		req, err := newRequest(bufferedReader)
		if err != nil {
			res = &response{
				Version: "2.0",
				Error: &Error{
					Code:    InvalidRequest,
					Message: err.Error(),
				},
			}
		} else {
			res = s.handleRequest(req)
		}

		return json.Marshal(res)
	} else {
		batchReq := batch{}
		batchRes := []json.RawMessage{}

		if err := json.NewDecoder(bufferedReader).Decode(&batchReq); err != nil {
			return nil, err
		}

		for _, rawReq := range batchReq {
			if res, err := s.Handle(rawReq); err == nil { // todo: handle async
				batchRes = append(batchRes, res)
			}
		}

		return json.Marshal(batchRes)
	}
}

func isBatch(reader *bufio.Reader) bool {
	for {
		char, err := reader.ReadByte()
		if err != nil {
			break
		} else if char == ' ' || char == '\t' || char == '\r' || char == '\n' {
			continue
		} else {
			reader.UnreadByte()
			return char == '['
		}
	}

	return false
}

func isNil(i interface{}) bool {
	return i == nil || reflect.ValueOf(i).IsNil()
}

func (s *Server) handleRequest(req *request) *response {
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
		return res
	}

	args, err := buildArguments(req.Params, calledMethod.Handler, calledMethod.Params)
	if err != nil {
		res.Error = &Error{
			Code:    InvalidParams,
			Message: err.Error(),
		}
		return res
	}

	tuple := reflect.ValueOf(calledMethod.Handler).Call(args)
	if errAny := tuple[1].Interface(); !isNil(errAny) {
		res.Error = errAny.(*Error)
		return res
	}

	res.Result = tuple[0].Interface()
	return res
}

func buildArguments(params, handler any, configuredParams []Parameter) ([]reflect.Value, error) {
	args := []reflect.Value{}
	handlerType := reflect.TypeOf(handler)
	paramCount := handlerType.NumIn()
	paramsKind := reflect.TypeOf(params).Kind()

	for idx := 0; idx < paramCount; idx++ {
		valueContainer := reflect.New(handlerType.In(idx))
		var requestValue any
		found := true

		switch paramsKind {
		case reflect.Slice:
			paramsList := params.([]any)
			if len(paramsList) != paramCount {
				return nil, errors.New("missing param in list")
			}

			requestValue = paramsList[idx]
		case reflect.Map:
			paramsMap := params.(map[string]any)
			if len(configuredParams) != paramCount {
				return nil, errors.New("missing param name")
			}

			requestValue, found = paramsMap[configuredParams[idx].Name]
		default:
			return nil, errors.New("impossible param type: check request.isSane")
		}

		if found {
			valueMarshaled, err := json.Marshal(requestValue)
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(valueMarshaled, valueContainer.Interface())
			if err != nil {
				return nil, err
			}
		} else if !configuredParams[idx].Optional {
			return nil, errors.New("missing non-optional param")
		}

		args = append(args, valueContainer.Elem())
	}

	return args, nil
}

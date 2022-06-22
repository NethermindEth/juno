package rpcnew

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

var (
	errType     = reflect.TypeOf((*error)(nil)).Elem()
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

	ErrCreateServiceError = errors.New("create service error")
)

// RpcService  is a group of RPC funtions behaind the same service name and
// receiver.
type RpcService struct {
	name     string
	handlers map[string]*handler
	mu       sync.RWMutex
}

// NewRpcService returns a new RpcService instance with the given name, and
// using receiver as the struct wich contains all the methods will be the RPCs.
func NewRpcService(name string, receiver interface{}) (*RpcService, error) {
	receiverV := reflect.ValueOf(receiver)
	receiverT := receiverV.Type()
	// Check receiver type
	elemT := receiverT
	if elemT.Kind() == reflect.Pointer {
		elemT = elemT.Elem()
	}
	if elemT.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: %s", ErrCreateServiceError, "invalid receiver type")
	}

	s := &RpcService{
		handlers: make(map[string]*handler),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Build handlers
	for i := 0; i < receiverV.NumMethod(); i++ {
		method := receiverT.Method(i)
		// Discard methods declared on another package and the not exported too
		if method.PkgPath != "" || !method.IsExported() {
			continue
		}
		h, err := NewHandler(receiverV, method)
		if err != nil {
			return nil, err
		}
		s.handlers[makeHandlerName(name, method.Name)] = h
	}
	return s, nil
}

func makeHandlerName(serviceName, methodName string) string {
	return serviceName + "_" + strings.ToLower(methodName[:1]) + methodName[1:]
}

// handler represents an RPC function
type handler struct {
	receiver   reflect.Value
	function   reflect.Value
	arguments  reflect.Type
	hasContext bool
}

func NewHandler(receiver reflect.Value, method reflect.Method) (*handler, error) {
	h := &handler{
		receiver: receiver,
		function: method.Func,
	}
	// Check method output types
	if err := checkOutputs(method); err != nil {
		return nil, err
	}
	// Check method inputs type
	withContext, paramType, err := checkInputs(method)
	if err != nil {
		return nil, err
	}
	h.hasContext = withContext
	h.arguments = paramType
	return h, nil
}

func checkOutputs(method reflect.Method) error {
	methodT := method.Type
	numOut := methodT.NumOut()
	if numOut != 2 {
		return errors.New("unexpected number of outputs, want 2")
	}
	// Check if second output is of type error
	secondOut := methodT.Out(1)
	if !errType.AssignableTo(secondOut) {
		return errors.New("second type must be en error type")
	}
	return nil
}

func checkInputs(method reflect.Method) (withContext bool, paramType reflect.Type, err error) {
	numIn := method.Type.NumIn()

	switch numIn {
	case 3:
		firstIn := method.Type.In(1)
		if !contextType.AssignableTo(firstIn) {
			return false, nil, errors.New("if the function has two params the first param MUST be a context")
		}
		secondIn := method.Type.In(2)
		if secondIn.Kind() != reflect.Struct {
			return false, nil, errors.New("if the function has two params the second param MUST be a struct")
		}
		// With Context and RPC params
		return true, secondIn, nil
	case 2:
		firstIn := method.Type.In(1)
		if contextType.AssignableTo(firstIn) {
			// With context and without RPC params
			return true, nil, nil
		} else if firstIn.Kind() == reflect.Struct {
			// Without Context and with strcut param
			return false, firstIn, nil
		} else {
			// Invalid param
			return false, nil, errors.New("function with one param MUST be a Context or a struct")
		}
	case 1:
		// Without Context and param
		return false, nil, nil
	default:
		return false, nil, errors.New("number of inputs must to be 2 at max")
	}
}

func (h *handler) Call(ctx context.Context, params reflect.Value) (interface{}, error) {
	var out []reflect.Value
	if h.hasContext {
		out = h.function.Call([]reflect.Value{h.receiver, reflect.ValueOf(ctx), params})
	} else {
		out = h.function.Call([]reflect.Value{h.receiver, params})
	}
	outValue := out[0]
	outError := out[1]
	if outError.IsNil() {
		return outValue.Interface(), nil
	}
	return outValue.Interface(), outError.Interface().(error)
}

func (s *RpcService) Call(ctx context.Context, request RpcRequest) (*RpcResponse, error) {
	// Lock/Unlock the service mutex
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if the method exists
	h, ok := s.handlers[request.Method]
	if !ok {
		return nil, NewErrMethodNotFound(request.Method)
	}

	var param reflect.Value
	if isBatch(request.Params) {
		// Parsing params by-position
		var positionalParams []json.RawMessage
		err := json.Unmarshal(request.Params, &positionalParams)
		if err != nil {
			return nil, NewErrInvalidParams("invalid by-position params")
		}
		param, err = h.parsePositionalParams(positionalParams)
		if err != nil {
			return nil, err
		}
	} else {
		// Parsing params by-name
		var namedParams map[string]json.RawMessage
		err := json.Unmarshal(request.Params, &namedParams)
		if err != nil {
			return nil, NewErrInvalidParams("unmarshal by-name params error")
		}
		param, err = h.parseNamedParams(namedParams)
		if err != nil {
			return nil, err
		}
	}

	out, err := h.Call(ctx, param)
	if err != nil {
		return nil, err
	}
	return NewRpcResponse(request.Id, out), nil
}

func (h handler) parsePositionalParams(params []json.RawMessage) (reflect.Value, error) {
	param := reflect.New(h.arguments).Elem()
	numField := param.NumField()
	if len(params) != numField {
		return reflect.Value{}, NewErrInvalidParams(fmt.Sprintf("invalid number of by-position params, the called function needs %d params", numField))
	}
	for i := 0; i < numField; i++ {
		field := param.Field(i)
		fieldV := reflect.New(field.Type())
		err := json.Unmarshal(params[i], fieldV.Interface())
		if err != nil {
			return reflect.Value{}, NewErrInvalidParams(fmt.Sprintf("invalid param at index %d", i))
		}
		field.Set(fieldV.Elem())
	}
	return param, nil
}

func (h handler) parseNamedParams(params map[string]json.RawMessage) (reflect.Value, error) {
	param := reflect.New(h.arguments).Elem()
	numField := param.NumField()
	if numField != len(params) {
		return reflect.Value{}, NewErrInvalidParams(fmt.Sprintf("invalid number of by-name params, the called function needs %d params", numField))
	}
	for k, v := range params {
		field := param.FieldByName(k)
		if !field.IsValid() {
			return reflect.Value{}, NewErrInvalidParams(fmt.Sprintf("invalid param name %s", k))
		}
		fieldV := reflect.New(field.Type())
		err := json.Unmarshal(v, fieldV.Interface())
		if err != nil {
			return reflect.Value{}, NewErrInvalidParams(fmt.Sprintf("invalid param name %s", k))
		}
		field.Set(fieldV.Elem())
	}
	return param, nil
}

func (s *RpcService) ProcessOut(out interface{}, err error) (*RpcResponse, error) {
	return &RpcResponse{
		Id:      "",
		Jsonrpc: JsonRpcVersion,
		Result:  out,
		Error:   nil,
	}, err
}

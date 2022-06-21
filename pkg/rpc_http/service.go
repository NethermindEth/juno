package rpcnew

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
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
}

// NewRpcService returns a new RpcService instance with the given name, and
// using receiver as the struct wich contains all the methods will be the RPCs.
func NewRpcService(name string, receiver interface{}) (*RpcService, error) {
	receiverV := reflect.ValueOf(receiver)
	receiverT := receiverV.Type()
	// Check receiver type
	if receiverT.Kind() == reflect.Pointer {
		elemT := receiverT.Elem()
		if elemT.Kind() != reflect.Struct {
			return nil, fmt.Errorf("%w: %s", ErrCreateServiceError, "invalid receiver type")
		}
	}

	s := &RpcService{
		handlers: make(map[string]*handler),
	}
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

func (h *handler) Call(v ...reflect.Value) (interface{}, error) {
	out := h.function.Call(append([]reflect.Value{h.receiver}, v...))
	outValue := out[0]
	outError := out[1]
	if outError.IsNil() {
		return outValue.Interface(), nil
	}
	return outValue.Interface(), outError.Interface().(error)
}

func (s *RpcService) Call(ctx context.Context, request RpcRequest) (*RpcResponse, error) {
	handler, ok := s.handlers[request.Method]
	if !ok {
		return nil, NewErrMethodNotFound(request.Method)
	}
	decoder := json.NewDecoder(bytes.NewReader(request.Params))
	token, err := decoder.Token()
	if err != nil {
		// TODO: Check empty params
		return nil, err
	}
	var param reflect.Value
	if token == json.Delim('[') {
		// Parsing params by-position
		var positionalParams []json.RawMessage
		err := json.Unmarshal(request.Params, &positionalParams)
		if err != nil {
			return nil, NewErrInvalidRequest("invalid by-position params")
		}
		param = reflect.New(handler.arguments).Elem()
		numField := param.NumField()
		if len(positionalParams) != numField {
			return nil, NewErrInvalidRequest(fmt.Sprintf("invalid number of by-position params, the called function needs %d params", numField))
		}
		for i := 0; i < numField; i++ {
			field := param.Field(i)
			// TODO: Check if IsValid and CanSet
			fieldV := reflect.New(field.Type())
			err := json.Unmarshal(positionalParams[i], fieldV.Interface())
			if err != nil {
				return nil, NewErrInvalidRequest(fmt.Sprintf("invalid param at index %d", i))
			}
			field.Set(fieldV.Elem())
		}
	} else if token == json.Delim('{') {
		// Parsing params by-name
		param = reflect.New(handler.arguments)
		err := json.Unmarshal(request.Params, param.Interface())
		if err != nil {
			// TODO: send more context about the error
			return nil, NewErrInvalidRequest("unmarshal by-name params error")
		}
		param = param.Elem()
	} else {
		return nil, NewErrInvalidRequest("invalid param format")
	}
	if handler.hasContext {
		out, err := handler.Call(reflect.ValueOf(ctx), param)
		response, err := s.ProcessOut(out, err)
		response.Id = request.Id
		return response, err
	} else {
		out, err := handler.Call(param)
		response, err := s.ProcessOut(out, err)
		response.Id = request.Id
		return response, err
	}
}

func (s *RpcService) ProcessOut(out interface{}, err error) (*RpcResponse, error) {
	return &RpcResponse{
		Id:      "",
		Jsonrpc: JsonRpcVersion,
		Result:  out,
		Error:   nil,
	}, err
}

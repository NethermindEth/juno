package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// service is a JSON-RPC service that represents a collection of methods, which names are prefixed
// with the service name.
type rpcService struct {
	Name    string
	Methods map[string]*rpcMethod
}

func newRpcService(name string, receiver interface{}) (*rpcService, error) {
	// Check if service is a pointer to a struct
	if !isPointerToStruct(reflect.TypeOf(receiver)) {
		return nil, errors.New("receiver must be a pointer to a struct")
	}
	receiverV := reflect.ValueOf(receiver)
	// Check if service has at least one method
	if receiverV.NumMethod() == 0 {
		return nil, errors.New("service must have at least one method")
	}
	serviceObject := &rpcService{
		Name: name,
	}
	// Scan service methods
	for i := 0; i < receiverV.NumMethod(); i++ {
		method := receiverV.Type().Method(i)
		if method.PkgPath != "" {
			continue
		}
		// Check if method is exported
		if !method.IsExported() {
			continue
		}
		// Check method argument
		if err := checkMethodArguments(method); err != nil {
			return nil, err
		}
		// Check method output
		if err := checkMethodOutput(method); err != nil {
			return nil, err
		}
		// Create method
		err := serviceObject.AddMethod(receiver, method)
		if err != nil {
			return nil, err
		}
	}
	return serviceObject, nil
}

func (s *rpcService) Call(request *rpcRequest) (interface{}, error) {
	method, ok := s.Methods[request.Method]
	if !ok {
		return nil, errMethodNotFound
	}
	result, err := method.Call(request.Params)
	if err != nil {
		return nil, err
	}
	if request.IsNotification() {
		return nil, nil
	}
	return result, nil
}

func (s *rpcService) AddMethod(receiver interface{}, m reflect.Method) error {
	var methodName string
	if s.Name != "" {
		methodName = s.Name + "_" + strings.ToLower(m.Name[:1]) + m.Name[1:]
	} else {
		methodName = strings.ToLower(m.Name[:1]) + m.Name[1:]
	}
	// Check if the method already exists
	if _, ok := s.Methods[methodName]; ok {
		return errors.New(fmt.Sprintf("method %s already exists", methodName))
	}
	if s.Methods == nil {
		s.Methods = make(map[string]*rpcMethod)
	}
	// Create method
	s.Methods[methodName] = newRpcMethod(methodName, receiver, m, m.Type.NumIn() > 2)
	return nil
}

type rpcMethod struct {
	Name      string
	Receiver  reflect.Value
	Method    reflect.Method
	HasParams bool
	ParamT    reflect.Type
}

func newRpcMethod(name string, receiver interface{}, m reflect.Method, hasParams bool) *rpcMethod {
	var paramT reflect.Type
	if hasParams {
		paramT = m.Type.In(2).Elem()
	}
	return &rpcMethod{
		Name:      name,
		Receiver:  reflect.ValueOf(receiver),
		Method:    m,
		HasParams: hasParams,
		ParamT:    paramT,
	}
}

func (m *rpcMethod) Call(params json.RawMessage) (interface{}, error) {
	if !m.HasParams {
		if params != nil {
			return nil, errInvalidParams
		}
		return m.call(reflect.Value{})
	}
	if isBatchRequest(params) {
		// Batch request
		var paramsArray []json.RawMessage
		if err := json.Unmarshal(params, &paramsArray); err != nil {
			return nil, errInvalidParams
		}
		// Cehck if the number of parameters is correct
		if m.ParamT.NumField() != len(paramsArray) {
			return nil, errInvalidParams
		}
		// Build param object
		paramObject := reflect.New(m.ParamT)
		for i, param := range paramsArray {
			// Unmarshal parameter
			paramField := paramObject.Elem().Field(i)
			if err := json.Unmarshal(param, paramField.Addr().Interface()); err != nil {
				return nil, errInvalidParams
			}
		}
		return m.call(paramObject)

	} else {
		// By name request
		var paramsMap map[string]json.RawMessage
		if err := json.Unmarshal(params, &paramsMap); err != nil {
			return nil, errInvalidParams
		}
		// Check if the number of parameters is correct
		if m.ParamT.NumField() != len(paramsMap) {
			return nil, errInvalidParams
		}
		// Build param object
		paramObject := reflect.New(m.ParamT)
		for name, param := range paramsMap {
			// Unmarshal parameter
			paramField := paramObject.Elem().FieldByName(name)
			if !paramField.IsValid() {
				return nil, errInvalidParams
			}
			if err := json.Unmarshal(param, paramField.Addr().Interface()); err != nil {
				return nil, errInvalidParams
			}
		}
		return m.call(paramObject)
	}
}

func (m *rpcMethod) call(paramObject reflect.Value) (interface{}, error) {
	// Call method
	// TODO: pass the correct Context. Maybe the context should com from the request?
	var result []reflect.Value
	if m.HasParams {
		result = m.Method.Func.Call([]reflect.Value{m.Receiver, reflect.ValueOf(context.Background()), paramObject})
	} else {
		result = m.Method.Func.Call([]reflect.Value{m.Receiver, reflect.ValueOf(context.Background())})
	}
	// Check if the number of return values is correct
	if len(result) != 2 {
		return nil, fmt.Errorf("%w: invalid number of return values", errInternalError)
	}
	// Check if the return value is an error
	if !result[1].IsNil() {
		return nil, result[1].Interface().(error)
	}
	// Return result
	return result[0].Interface(), nil
}

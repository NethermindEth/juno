package jsonrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

var (
	interfaceType = reflect.TypeOf((*interface{})(nil)).Elem()
	errorType     = reflect.TypeOf((*error)(nil)).Elem()

	jsonrpcVersion = "2.0"

	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidFunc   = errors.New("invalid function")
)

type endpoint struct {
	method     string
	paramNames []string
	paramTypes []reflect.Type
	function   reflect.Value
}

type JsonRpc struct {
	endpoints map[string]*endpoint
}

func NewJsonRpc() *JsonRpc {
	return &JsonRpc{
		endpoints: make(map[string]*endpoint),
	}
}

func (s *JsonRpc) RegisterFunc(name string, handler any, paramNames ...string) error {
	if _, ok := s.endpoints[name]; ok {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, name)
	}
	handlerT := reflect.TypeOf(handler)
	if handlerT.Kind() != reflect.Func {
		return fmt.Errorf("%w: %s", ErrInvalidFunc, "handler must be a function")
	}
	if handlerT.NumIn() != len(paramNames) {
		return fmt.Errorf("%w: %s", ErrInvalidFunc, "number of function params and param names must match")
	}
	if err := checkHandlerOutput(handlerT); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidFunc, err.Error())
	}
	e := &endpoint{
		method:     name,
		paramNames: paramNames,
		paramTypes: make([]reflect.Type, 0, handlerT.NumIn()),
		function:   reflect.ValueOf(handler),
	}
	for i := 0; i < handlerT.NumIn(); i++ {
		e.paramTypes = append(e.paramTypes, handlerT.In(i))
	}
	s.endpoints[name] = e
	return nil
}

func (s *JsonRpc) Call(request []byte) json.RawMessage {
	if isJsonList(request) {
		var rawRequests []json.RawMessage
		if err := json.Unmarshal(request, &rawRequests); err != nil {
			return errResponse(nil, ErrParseError)
		}
		if len(rawRequests) == 0 {
			return errResponse(nil, ErrInvalidRequest)
		}
		responses := make([]json.RawMessage, 0, len(rawRequests))
		responseChannel := make(chan json.RawMessage)
		for _, rawRequest := range rawRequests {
			go func(rawRequest json.RawMessage, outChannel chan json.RawMessage) {
				outChannel <- s.callRaw(rawRequest)
			}(rawRequest, responseChannel)
		}
		for i := 0; i < len(rawRequests); i++ {
			response := <-responseChannel
			if response != nil {
				responses = append(responses, response)
			}
		}
		if len(responses) == 0 {
			// notest
			return nil
		}
		response, _ := json.Marshal(responses)
		return response
	}
	return s.callRaw(request)
}

func (s *JsonRpc) callRaw(request json.RawMessage) json.RawMessage {
	var requestObject rpcRequest
	if err := json.Unmarshal(request, &requestObject); err != nil {
		if _, ok := err.(*json.UnmarshalTypeError); ok {
			return errResponse(nil, ErrInvalidRequest)
		}
		if _, ok := err.(*json.SyntaxError); ok {
			return errResponse(nil, ErrParseError)
		}
		return errResponse(nil, err)
	}
	out, err := s.processRequest(&requestObject)
	if err != nil {
		return errResponse(requestObject.Id, err)
	}
	if out == nil {
		return nil
	}
	return sucessReponse(requestObject.Id, out)
}

func (s *JsonRpc) processRequest(request *rpcRequest) (any, error) {
	e, ok := s.endpoints[request.Method]
	if !ok {
		return nil, ErrMethodNotFound
	}
	params := make([]reflect.Value, 0, len(e.paramTypes))
	if isJsonList(request.Params) {
		var paramsRaw []json.RawMessage
		if err := json.Unmarshal(request.Params, &paramsRaw); err != nil {
			// XXX: Is this error even possible?
			return nil, err
		}
		if len(paramsRaw) > len(e.paramNames) {
			return nil, ErrInvalidParams
		}
		for i := 0; i < len(paramsRaw); i++ {
			params = append(params, reflect.New(e.paramTypes[i]).Elem())
			if err := json.Unmarshal(paramsRaw[i], params[i].Addr().Interface()); err != nil {
				return nil, ErrInvalidParams
			}
		}
		for i := len(paramsRaw); i < len(e.paramTypes); i++ {
			params = append(params, reflect.New(e.paramTypes[i]).Elem())
		}
	} else {
		var paramsRaw map[string]json.RawMessage
		if err := json.Unmarshal(request.Params, &paramsRaw); err != nil {
			return nil, ErrInvalidParams
		}
		for i := 0; i < len(e.paramNames); i++ {
			if param, ok := paramsRaw[e.paramNames[i]]; ok {
				params = append(params, reflect.New(e.paramTypes[i]).Elem())
				if err := json.Unmarshal(param, params[i].Addr().Interface()); err != nil {
					return nil, ErrInvalidParams
				}
			} else {
				params = append(params, reflect.New(e.paramTypes[i]).Elem())
			}
		}
	}
	callResult := e.function.Call(params)
	if !callResult[1].IsNil() {
		return nil, callResult[1].Interface().(error)
	}
	if request.IsNotification() {
		return nil, nil
	}
	return callResult[0].Interface(), nil
}

func errResponse(id any, err error) json.RawMessage {
	return newRpcResponse(id, nil, err)
}

func sucessReponse(id any, result any) json.RawMessage {
	return newRpcResponse(id, result, nil)
}

func newRpcResponse(id any, result any, err error) json.RawMessage {
	response := rpcResponse{
		Jsonrpc: jsonrpcVersion,
		Id:      id,
		Error:   err,
		Result:  result,
	}
	rawResponse, err := json.Marshal(&response)
	if err != nil {
		// notest
		rawResponse, _ = json.Marshal(&rpcResponse{
			Jsonrpc: jsonrpcVersion,
			Id:      id,
			Result:  nil,
			Error:   ErrInternalError,
		})
	}
	return rawResponse
}

func isJsonList(request json.RawMessage) bool {
	for _, c := range request {
		// skip insignificant whitespace (http://www.ietf.org/rfc/rfc4627.txt)
		if c == 0x20 || c == 0x09 || c == 0x0a || c == 0x0d {
			// notest
			continue
		}
		return c == '['
	}
	// notest
	return false
}

func checkHandlerOutput(handlerT reflect.Type) error {
	if handlerT.NumOut() != 2 {
		return errors.New("handler must return 2 values")
	}
	if handlerT.Out(0) != interfaceType {
		return errors.New("first return value must be an interface")
	}
	if handlerT.Out(1) != errorType {
		return errors.New("second return value must be an error")
	}
	return nil
}

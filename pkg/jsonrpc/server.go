package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

var (
	contextType   = reflect.TypeOf((*context.Context)(nil)).Elem()
	interfaceType = reflect.TypeOf((*interface{})(nil)).Elem()
	errorType     = reflect.TypeOf((*error)(nil)).Elem()

	jsonrpc = "2.0"
)

// Server is the JSON-RPC 2.0 server. It can register services and handle requests. Also, it can support middleware.
type Server struct {
	services   map[string]*rpcService
	middleware Middleware
}

type (
	// RequestProcessor is the type of function that processes a request. It is use by the middleware to send the requests
	// to the server.
	RequestProcessor func(request *rpcRequest) (any, error)
	// Middleware brinds the ability to access the request and the response before and after the call to the service.
	Middleware func(request *rpcRequest, processor RequestProcessor) (any, error)
)

// NewServer creates a new JSON-RPC 2.0 server instance.
func NewServer() *Server {
	return &Server{}
}

// RegisterService registers a service on the server. If the service already exists, it returns an error. If the service
// does not fit the requirements, it returns an error.
func (s *Server) RegisterService(name string, receiver any) error {
	// Check if service already exists
	if _, ok := s.services[name]; ok {
		return errors.New("service already exists")
	}
	serv, err := newRpcService(name, receiver)
	if err != nil {
		return err
	}
	// Init services map
	if s.services == nil {
		s.services = make(map[string]*rpcService)
	}
	// Add service
	s.services[name] = serv
	return nil
}

// SetMiddleware sets the middleware to be used by the server. If is called more than once,
// the previous middleware will be replaced.
func (s *Server) SetMiddleware(middleware Middleware) {
	s.middleware = middleware
}

// Call calls the method of the given name on the proper service. Request must be a valid JSON-RPC 2.0 request,
// can be either a batch or a single request. If any error happens, the error is sent to the client on the response.
func (s *Server) Call(request json.RawMessage) json.RawMessage {
	// Check if the request is a valid JSON
	var temp json.RawMessage
	if err := json.Unmarshal(request, &temp); err != nil {
		return buildResponse(nil, nil, errParseError)
	}
	request = temp
	// Check if request is a batch
	if isBatchRequest(request) {
		var rawRequests []json.RawMessage
		if err := json.Unmarshal(request, &rawRequests); err != nil {
			// TODO: maybe this error as an internal error because the request is a valid JSON and it is a batch
			return buildResponse(nil, nil, errParseError)
		}
		if len(rawRequests) == 0 {
			return buildResponse(nil, nil, errInvalidRequest)
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
			return nil
		}
		response, _ := json.Marshal(responses)
		return response
	}
	return s.callRaw(request)
}

func (s *Server) callRaw(request json.RawMessage) json.RawMessage {
	var requestObject rpcRequest
	if err := json.Unmarshal(request, &requestObject); err != nil {
		return buildResponse(nil, nil, err)
	}
	// Call method
	out, err := s.call(&requestObject)
	if err != nil {
		return buildResponse(requestObject.Id, nil, err)
	}
	if out == nil {
		// Notification
		return nil
	}
	return buildResponse(requestObject.Id, out, nil)
}

func (s *Server) call(request *rpcRequest) (out any, err error) {
	if s.middleware != nil {
		// Process request through middleware
		out, err = s.middleware(request, s.processRequest)
	} else {
		// Process request directly
		out, err = s.processRequest(request)
	}
	if err != nil {
		return nil, err
	}
	if out == nil {
		// Notification
		return nil, nil
	}
	return out, nil
}

func (s *Server) processRequest(request *rpcRequest) (any, error) {
	service, ok := s.services[getServiceName(request.Method)]
	if !ok {
		return nil, errMethodNotFound
	}
	return service.Call(request)
}

func buildResponse(id any, result any, err error) json.RawMessage {
	response := rpcResponse{
		Jsonrpc: jsonrpc,
		Id:      id,
		Result:  result,
	}

	if err != nil {
		if errors.Is(err, errInvalidRequest) {
			response.Error = newErrInvalidRequest(nil)
		} else if errors.Is(err, errMethodNotFound) {
			response.Error = newErrMethodNotFound(nil)
		} else if errors.Is(err, errParseError) {
			response.Error = newErrParseError(nil)
		} else if errors.Is(err, errInvalidParams) {
			response.Error = newErrInvalidParams(nil)
		} else {
			response.Error = newErrInternalError(nil)
		}
	}

	responseByte, err := json.Marshal(&response)
	if err != nil {
		responseByte, _ = json.Marshal(&rpcResponse{
			Jsonrpc: jsonrpc,
			Id:      id,
			Result:  nil,
			Error:   newErrInternalError(nil),
		})
	}
	return responseByte
}

func getServiceName(method string) string {
	sepIndex := strings.Index(method, "_")
	if sepIndex == -1 {
		return ""
	}
	return method[:sepIndex]
}

func isBatchRequest(request json.RawMessage) bool {
	for _, c := range request {
		// skip insignificant whitespace (http://www.ietf.org/rfc/rfc4627.txt)
		if c == 0x20 || c == 0x09 || c == 0x0a || c == 0x0d {
			continue
		}
		return c == '['
	}
	return false
}

func checkMethodArguments(method reflect.Method) error {
	numIn := method.Type.NumIn()
	if numIn < 2 || numIn > 3 {
		return errors.New("invalid method signature")
	}
	// Check if first argument is a context.Context
	if method.Type.In(1) != contextType {
		return errors.New("first argument must be a context.Context")
	}
	// Check if second argument is a pointer to a struct
	if numIn == 3 && !isPointerToStruct(method.Type.In(2)) {
		return errors.New("second argument must be a pointer to a struct")
	}
	return nil
}

func isPointerToStruct(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct
}

func checkMethodOutput(method reflect.Method) error {
	// Check if method returns an interface and an error
	if method.Type.NumOut() != 2 {
		return errors.New("method must return two values")
	}
	// Check first output value is an interface
	if method.Type.Out(0) != interfaceType {
		return errors.New("first return value must be an interface")
	}
	// Check second output value is an error
	if method.Type.Out(1) != errorType {
		return errors.New("second return value must be an error")
	}
	return nil
}

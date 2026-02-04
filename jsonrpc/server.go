// Package jsonrpc implements a JSONRPC2.0 compliant server as described in https://www.jsonrpc.org/specification
package jsonrpc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc/pool"
	"go.uber.org/zap"
)

const (
	InvalidJSON    = -32700 // Invalid JSON was received by the server.
	InvalidRequest = -32600 // The JSON sent is not a valid Request object.
	MethodNotFound = -32601 // The method does not exist / is not available.
	InvalidParams  = -32602 // Invalid method parameter(s).
	InternalError  = -32603 // Internal JSON-RPC error.
)

var (
	ErrInvalidID = errors.New("id should be a string or an integer")

	bufferSize       = 128
	contextInterface = reflect.TypeOf((*context.Context)(nil)).Elem()
)

type Request struct {
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

// CloneWithData copies the error and sets the data field on the copy
func (e *Error) CloneWithData(data any) *Error {
	dup := *e
	dup.Data = data
	return &dup
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
		return &Error{Code: InternalError, Message: "Internal error", Data: data}
	}
}

func (r *Request) isSane() error {
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

	// The method takes a context as its first parameter.
	// Set upon successful registration.
	needsContext bool

	// The number of required parameters in the method.
	// Set upon successful registration.
	requiredParamCount int
}

type Server struct {
	methods              map[string]Method
	validator            Validator
	pool                 *pool.Pool
	log                  utils.StructuredLogger
	listener             EventListener
	disableBatchRequests bool
}

type Validator interface {
	Struct(any) error
}

// NewServer instantiates a JSONRPC server
func NewServer(poolMaxGoroutines int, log utils.StructuredLogger) *Server {
	s := &Server{
		log:      log,
		methods:  make(map[string]Method),
		pool:     pool.New().WithMaxGoroutines(poolMaxGoroutines),
		listener: &SelectiveListener{},
	}

	return s
}

// WithValidator registers a validator to validate handler struct arguments
func (s *Server) WithValidator(validator Validator) *Server {
	s.validator = validator
	return s
}

// WithListener registers an EventListener
func (s *Server) WithListener(listener EventListener) *Server {
	s.listener = listener
	return s
}

// DisableBatchRequests disables batch JSON-RPC requests to the server
func (s *Server) DisableBatchRequests(forbid bool) *Server {
	s.disableBatchRequests = forbid
	return s
}

// RegisterMethods verifies and creates an endpoint that the server recognises.
//
// - name is the method name
// - handler is the function to be called when a request is received for the
// associated method. It should have (any, *jsonrpc.Error) as its return type
// - paramNames are the names of parameters in the order that they are expected
// by the handler
func (s *Server) RegisterMethods(methods ...Method) error {
	for idx := range methods {
		if err := s.registerMethod(methods[idx]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) registerMethod(method Method) error {
	handlerT := reflect.TypeOf(method.Handler)
	if handlerT.Kind() != reflect.Func {
		return errors.New("handler must be a function")
	}
	numArgs := handlerT.NumIn()
	if numArgs > 0 {
		if handlerT.In(0).Implements(contextInterface) {
			numArgs--
			method.needsContext = true
		}
	}
	if numArgs != len(method.Params) {
		return errors.New("number of non-context function params and param names must match")
	}
	outSize := handlerT.NumOut()
	if outSize < 2 || outSize > 3 {
		return errors.New("handler must return 2 or 3 values")
	}
	if outSize == 2 && handlerT.Out(1) != reflect.TypeOf(&Error{}) {
		return errors.New("second return value must be a *jsonrpc.Error for 2 tuple handler")
	} else if outSize == 3 && handlerT.Out(2) != reflect.TypeOf(&Error{}) {
		return errors.New("third return value must be a *jsonrpc.Error for 3 tuple handler")
	}

	if outSize == 3 && handlerT.Out(1) != reflect.TypeOf(http.Header{}) {
		return errors.New("second return value must be a http.Header for 3 tuple handler")
	}

	requiredParamCount := 0
	for _, param := range method.Params {
		if !param.Optional {
			requiredParamCount++
		}
	}
	method.requiredParamCount = requiredParamCount

	// The method is valid. Mutate the appropriate fields and register on the server.
	s.methods[method.Name] = method

	return nil
}

type Conn interface {
	io.Writer
	Equal(Conn) bool
}

type connection struct {
	w         io.Writer
	activated <-chan struct{}

	// initialErr is not thread-safe. It must be set to its final value before the connection is activated.
	initialErr error
}

var _ Conn = (*connection)(nil)

func (c *connection) Write(p []byte) (int, error) {
	<-c.activated
	if c.initialErr != nil {
		return 0, fmt.Errorf("there was an error while writing the initial response: %w", c.initialErr)
	}
	return c.w.Write(p)
}

func (c *connection) Equal(other Conn) bool {
	c2, ok := other.(*connection)
	if !ok {
		return false
	}
	return c.w == c2.w
}

// ConnKey the key used to retrieve the connection from the context passed to a handler.
// It is exported to allow transports to set it manually if they decide not to use HandleReadWriter, which sets it automatically.
// Manually setting the connection can be especially useful when testing handlers.
type ConnKey struct{}

// ConnFromContext returns a writable connection. The connection should
// be written in a separate goroutine, since writes from handlers are
// blocked until the initial response is sent.
func ConnFromContext(ctx context.Context) (Conn, bool) {
	conn := ctx.Value(ConnKey{})
	if conn == nil {
		return nil, false
	}
	w, ok := conn.(Conn)
	return w, ok
}

// HandleReadWriter permits methods to send messages on the connection after the server sends the initial response.
// rw must permit concurrent writes.
// A non-nil error indicates the initial response could not be sent, and that no method will be able to write the connection.
func (s *Server) HandleReadWriter(ctx context.Context, rw io.ReadWriter) error {
	activated := make(chan struct{})
	defer close(activated)
	conn := &connection{
		w:         rw.(io.Writer),
		activated: activated,
	}
	msgCtx := context.WithValue(ctx, ConnKey{}, conn)
	// header is unnecessary for read-writer(websocket)
	resp, _, err := s.HandleReader(msgCtx, rw)
	if err != nil {
		conn.initialErr = err
		return err
	}
	if resp != nil {
		if _, err = rw.Write(resp); err != nil {
			conn.initialErr = err
			return err
		}
	}
	return nil
}

// HandleReader processes a request to the server
// It returns the response in a byte array, only returns an
// error if it can not create the response byte array
func (s *Server) HandleReader(ctx context.Context, reader io.Reader) ([]byte, http.Header, error) {
	bufferedReader := bufio.NewReaderSize(reader, bufferSize)
	requestIsBatch := isBatch(bufferedReader)
	resp := &response{
		Version: "2.0",
	}

	header := http.Header{}

	dec := json.NewDecoder(bufferedReader)
	dec.UseNumber()

	if !requestIsBatch {
		req := new(Request)
		if jsonErr := dec.Decode(req); jsonErr != nil {
			resp.Error = Err(InvalidJSON, jsonErr.Error())
		} else if resObject, httpHeader, handleErr := s.handleRequest(ctx, req); handleErr != nil {
			if !errors.Is(handleErr, ErrInvalidID) {
				resp.ID = req.ID
			}
			resp.Error = Err(InvalidRequest, handleErr.Error())
			header = httpHeader
		} else {
			resp = resObject
			header = httpHeader
		}
	} else if !s.disableBatchRequests {
		var batchReq []json.RawMessage

		if batchJSONErr := dec.Decode(&batchReq); batchJSONErr != nil {
			resp.Error = Err(InvalidJSON, batchJSONErr.Error())
		} else if len(batchReq) == 0 {
			resp.Error = Err(InvalidRequest, "empty batch")
		} else {
			return s.handleBatchRequest(ctx, batchReq)
		}
	} else {
		resp.Error = Err(InvalidRequest, "batch requests are disabled")
	}

	if resp == nil {
		return nil, header, nil
	}

	result, err := json.Marshal(resp)
	return result, header, err
}

func (s *Server) handleBatchRequest(ctx context.Context, batchReq []json.RawMessage) ([]byte, http.Header, error) {
	var (
		mutex     sync.Mutex
		responses []json.RawMessage
		headers   []http.Header
	)

	addResponse := func(response any, header http.Header) {
		if responseJSON, err := json.Marshal(response); err != nil {
			s.log.Error("failed to marshal response", zap.Error(err))
		} else {
			mutex.Lock()
			responses = append(responses, responseJSON)
			headers = append(headers, header)
			mutex.Unlock()
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for _, rawReq := range batchReq {
		reqDec := json.NewDecoder(bytes.NewBuffer(rawReq))
		reqDec.UseNumber()

		req := new(Request)
		if err := reqDec.Decode(req); err != nil {
			addResponse(&response{
				Version: "2.0",
				Error:   Err(InvalidRequest, err.Error()),
			}, http.Header{})
			continue
		}

		wg.Add(1)
		s.pool.Go(func() {
			defer wg.Done()

			resp, header, err := s.handleRequest(ctx, req)
			if err != nil {
				resp = &response{
					Version: "2.0",
					Error:   Err(InvalidRequest, err.Error()),
				}
				if !errors.Is(err, ErrInvalidID) {
					resp.ID = req.ID
				}
			}
			// for notification request response is nil and header is irrelevant for now
			if resp != nil {
				addResponse(resp, header)
			}
		})
	}

	wg.Wait()

	// merge headers
	finalHeaders := http.Header{}
	for _, header := range headers {
		for k, v := range header {
			for _, e := range v {
				finalHeaders.Add(k, e)
			}
		}
	}

	// according to the spec if there are no response objects server must not return empty array
	if len(responses) == 0 {
		return nil, finalHeaders, nil
	}

	result, err := json.Marshal(responses)

	return result, finalHeaders, err // todo: fix batch request aggregate header
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

func isNilOrEmpty(i any) (bool, error) {
	if utils.IsNil(i) {
		return true, nil
	}

	switch reflect.TypeOf(i).Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return reflect.ValueOf(i).Len() == 0, nil
	default:
		return false, fmt.Errorf("impossible param type: check request.isSane")
	}
}

func (s *Server) handleRequest(ctx context.Context, req *Request) (*response, http.Header, error) {
	// TODO(structured-logging): migrate to explicit zap.Field values
	s.log.Trace("Received request", utils.SugaredFields("req", req)...)

	header := http.Header{}
	if err := req.isSane(); err != nil {
		s.log.Trace("Request sanity check failed", zap.Error(err))
		return nil, header, err
	}

	res := &response{
		Version: "2.0",
		ID:      req.ID,
	}

	calledMethod, found := s.methods[req.Method]
	if !found {
		res.Error = Err(MethodNotFound, nil)
		s.log.Trace("Method not found in request", zap.String("method", req.Method))
		return res, header, nil
	}

	handlerTimer := time.Now()
	s.listener.OnNewRequest(req.Method)
	args, err := s.buildArguments(ctx, req.Params, calledMethod)
	if err != nil {
		res.Error = Err(InvalidParams, err.Error())
		s.log.Trace("Error building arguments for RPC call", zap.Error(err))
		return res, header, nil
	}
	defer func() {
		s.listener.OnRequestHandled(req.Method, time.Since(handlerTimer))
	}()

	tuple := reflect.ValueOf(calledMethod.Handler).Call(args)
	if res.ID == nil { // notification
		s.log.Trace("Notification received, no response expected")
		return nil, header, nil
	}

	errorIndex := 1
	if len(tuple) == 3 {
		errorIndex = 2
		header = (tuple[1].Interface()).(http.Header)
	}

	if errAny := tuple[errorIndex].Interface(); !utils.IsNil(errAny) {
		res.Error = errAny.(*Error)
		if res.Error.Code == InternalError {
			s.listener.OnRequestFailed(req.Method, res.Error)
			reqJSON, _ := json.Marshal(req)
			errJSON, _ := json.Marshal(res.Error)
			s.log.Debug("Failed handing RPC request", zap.String("req", string(reqJSON)), zap.String("res", string(errJSON)))
		}
		return res, header, nil
	}
	res.Result = tuple[0].Interface()

	return res, header, nil
}

//nolint:gocyclo
func (s *Server) buildArguments(ctx context.Context, params any, method Method) ([]reflect.Value, error) {
	handlerType := reflect.TypeOf(method.Handler)

	numArgs := handlerType.NumIn()
	args := make([]reflect.Value, 0, numArgs)
	addContext := 0

	if method.needsContext {
		args = append(args, reflect.ValueOf(ctx))
		addContext = 1
	}

	isNilOrEmpty, err := isNilOrEmpty(params)
	if err != nil {
		return nil, err
	}

	if isNilOrEmpty {
		allParamsAreOptional := utils.All(method.Params, func(p Parameter) bool {
			return p.Optional
		})

		if len(method.Params) > 0 && !allParamsAreOptional {
			return nil, errors.New("missing non-optional param field")
		}

		for i := addContext; i < numArgs; i++ {
			arg := reflect.New(handlerType.In(i)).Elem()
			args = append(args, arg)
		}

		return args, nil
	}

	switch reflect.TypeOf(params).Kind() {
	case reflect.Slice:
		paramsList := params.([]any)

		// Ensure that the number of provided parameters is between required and total parameters
		if len(paramsList) < method.requiredParamCount || len(paramsList) > len(method.Params) {
			return nil, errors.New("missing/unexpected params in list")
		}

		for i, param := range paramsList {
			v, err := s.parseParam(param, handlerType.In(i+addContext))
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
		// Add remaining optional parameters if available
		for i := addContext + len(paramsList); i < numArgs; i++ {
			args = append(args, reflect.New(handlerType.In(i)).Elem())
		}
	case reflect.Map:
		paramsMap := params.(map[string]any)

		for i, configuredParam := range method.Params {
			var v reflect.Value
			if param, found := paramsMap[configuredParam.Name]; found {
				var err error
				v, err = s.parseParam(param, handlerType.In(i+addContext))
				if err != nil {
					return nil, err
				}

				delete(paramsMap, configuredParam.Name)
			} else if configuredParam.Optional {
				// optional parameter
				v = reflect.New(handlerType.In(i + addContext)).Elem()
			} else {
				return nil, errors.New("missing non-optional param: " + configuredParam.Name)
			}

			args = append(args, v)
		}

		// If there are any remaining parameters in the given parameters, it means that
		// there are extra junks in the request, which could be a typo. We return an error
		// to ensure that the request only contains the expected parameters.
		remainingKeys := make([]string, 0, len(paramsMap))
		for k := range paramsMap {
			remainingKeys = append(remainingKeys, k)
		}
		if len(remainingKeys) > 0 {
			return nil, errors.New("unexpected params: " + strings.Join(remainingKeys, ", "))
		}
	default:
		// Todo: consider returning InternalError
		return nil, errors.New("impossible param type: check request.isSane")
	}
	return args, nil
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
		for i := range param.Len() {
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

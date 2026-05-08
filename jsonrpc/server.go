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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/NethermindEth/juno/utils/jsonx"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/bytedance/sonic/option"
	"github.com/sourcegraph/conc/pool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// pretouchRecursiveDepth controls how many extra passes sonic.Pretouch
// makes over types that exceed its default inline depth (3 levels).
// With value 4, total covered depth is MaxInlineDepth(3)+4 = 7 levels —
// enough for any realistic Juno RPC type tree (transactions/traces nest
// at most 5–6 levels deep).
const pretouchRecursiveDepth = 4

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
	contextInterface = reflect.TypeFor[context.Context]()
)

type Request struct {
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      any             `json:"id,omitempty"`
}

// Params kind sentinels returned by paramsKind. Decoding params at the
// envelope level as json.RawMessage means we never materialize them as
// []any/map[string]any; the leading non-whitespace byte tells us how to
// route the per-slot decode.
const (
	paramsKindNone    byte = 'N' // missing, "null", or whitespace-only
	paramsKindArray   byte = 'A' // leading '['
	paramsKindObject  byte = 'O' // leading '{'
	paramsKindInvalid byte = 'X' // anything else (number, string, boolean)
)

func paramsKind(p json.RawMessage) byte {
	for i := range p {
		c := p[i]
		switch c {
		case ' ', '\t', '\r', '\n':
			continue
		case '[':
			return paramsKindArray
		case '{':
			return paramsKindObject
		case 'n':
			if len(p)-i >= 4 && p[i+1] == 'u' && p[i+2] == 'l' && p[i+3] == 'l' {
				return paramsKindNone
			}
			return paramsKindInvalid
		default:
			return paramsKindInvalid
		}
	}
	return paramsKindNone
}

// MarshalLogObject implements [zapcore.ObjectMarshaler].
func (r *Request) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("jsonrpc", log.SanitizeString(r.Version))
	enc.AddString("method", log.SanitizeString(r.Method))
	if r.ID != nil {
		err := enc.AddReflected("id", r.ID)
		if err != nil {
			return err
		}
	}
	if r.Params != nil {
		err := enc.AddReflected("params", r.Params)
		if err != nil {
			return err
		}
	}
	return nil
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

	if paramsKind(r.Params) == paramsKindInvalid {
		return errors.New("params should be an array or an object")
	}

	switch id := r.ID.(type) {
	case nil, string:
	case json.Number:
		if strings.Contains(id.String(), ".") {
			return ErrInvalidID
		}
	default:
		return ErrInvalidID
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

	// Per-method binding plan, populated once at registration so the
	// request hot path performs no reflection on the handler signature.
	//
	// inTypes / inZeroes are one entry per declared param (excluding
	// context); index matches Params[i]. inZeroes[i] is a cached zero
	// reflect.Value used for missing-optional slots. paramByName maps a
	// param name to its inTypes index for O(1) named-arg lookup.
	// handlerVal is reflect.ValueOf of the handler, cached so per-request
	// dispatch skips that conversion. needsValidation[i] is true iff the
	// i-th param's type (or anything reachable through it) carries a
	// `validate:` tag; tag-free types skip the validateParam recursion
	// entirely on the hot path.
	inTypes         []reflect.Type
	inZeroes        []reflect.Value
	paramByName     map[string]int
	handlerVal      reflect.Value
	needsValidation []bool
}

type Server struct {
	methods              map[string]Method
	validator            Validator
	pool                 *pool.Pool
	logger               log.StructuredLogger
	listener             EventListener
	disableBatchRequests bool
	pretouched           []reflect.Type
}

type Validator interface {
	Struct(any) error
}

// NewServer instantiates a JSONRPC server
func NewServer(poolMaxGoroutines int, logger log.StructuredLogger) *Server {
	s := &Server{
		logger:   logger,
		methods:  make(map[string]Method),
		pool:     pool.New().WithMaxGoroutines(poolMaxGoroutines),
		listener: &SelectiveListener{},
	}

	s.pretouch(reflect.TypeFor[Request]())
	s.pretouch(reflect.TypeFor[response]())
	s.pretouch(reflect.TypeFor[Error]())

	return s
}

// pretouch eagerly compiles sonic encode/decode paths for t. Failures
// (rare — only types containing channels/funcs as fields) are logged but
// do not block server startup. Each unique type is recorded once so
// callers can inspect what was pre-compiled via PretouchedTypes.
func (s *Server) pretouch(t reflect.Type) {
	if slices.Contains(s.pretouched, t) {
		return
	}
	s.pretouched = append(s.pretouched, t)
	if err := sonic.Pretouch(t, option.WithCompileRecursiveDepth(pretouchRecursiveDepth)); err != nil {
		s.logger.Warn("sonic pretouch failed", zap.String("type", t.String()), zap.Error(err))
	}
}

// PretouchedTypes returns the list of types submitted to sonic.Pretouch
// during construction and method registration, in registration order.
// Useful for verifying which encode/decode paths have been pre-compiled.
func (s *Server) PretouchedTypes() []reflect.Type {
	out := make([]reflect.Type, len(s.pretouched))
	copy(out, s.pretouched)
	return out
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
	if outSize == 2 && handlerT.Out(1) != errorType {
		return errors.New("second return value must be a *jsonrpc.Error for 2 tuple handler")
	} else if outSize == 3 && handlerT.Out(2) != errorType {
		return errors.New("third return value must be a *jsonrpc.Error for 3 tuple handler")
	}

	if outSize == 3 && handlerT.Out(1) != headerType {
		return errors.New("second return value must be a http.Header for 3 tuple handler")
	}

	requiredParamCount := 0
	for _, param := range method.Params {
		if !param.Optional {
			requiredParamCount++
		}
	}
	method.requiredParamCount = requiredParamCount

	// Build the per-request binding plan once. inTypes[i] is the slot
	// type for the i-th declared param (skipping context); the hot path
	// reads it as a plain slice index, never recomputes via reflect.
	addContext := 0
	if method.needsContext {
		addContext = 1
	}
	method.inTypes = make([]reflect.Type, len(method.Params))
	method.inZeroes = make([]reflect.Value, len(method.Params))
	method.needsValidation = make([]bool, len(method.Params))
	for i := range method.Params {
		t := handlerT.In(i + addContext)
		method.inTypes[i] = t
		method.inZeroes[i] = reflect.Zero(t)
		method.needsValidation[i] = typeHasValidateTag(t, nil)
	}
	if len(method.Params) > 0 {
		method.paramByName = make(map[string]int, len(method.Params))
		for i, p := range method.Params {
			method.paramByName[p.Name] = i
		}
	}
	method.handlerVal = reflect.ValueOf(method.Handler)

	// The method is valid. Mutate the appropriate fields and register on the server.
	s.methods[method.Name] = method

	s.pretouchHandlerTypes(handlerT, method.needsContext)

	return nil
}

var (
	errorType  = reflect.TypeFor[*Error]()
	headerType = reflect.TypeFor[http.Header]()
)

// typeHasValidateTag reports whether t — or anything reachable through
// its fields, slice/array elems, map values, or pointer indirection —
// carries a `validate` struct tag. Used at registration to decide
// whether the validateParam reflect walk needs to run on the hot path.
// Cycles are guarded via the visited set.
func typeHasValidateTag(t reflect.Type, visited map[reflect.Type]bool) bool {
	if t == nil {
		return false
	}
	if visited == nil {
		visited = make(map[reflect.Type]bool)
	}
	if visited[t] {
		return false
	}
	visited[t] = true

	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return typeHasValidateTag(t.Elem(), visited)
	case reflect.Map:
		return typeHasValidateTag(t.Key(), visited) || typeHasValidateTag(t.Elem(), visited)
	case reflect.Struct:
		for i := range t.NumField() {
			f := t.Field(i)
			if _, ok := f.Tag.Lookup("validate"); ok {
				return true
			}
			if typeHasValidateTag(f.Type, visited) {
				return true
			}
		}
	}
	return false
}

// pretouchHandlerTypes pre-compiles sonic encode/decode paths for every
// JSON-marshaled type the handler touches at request time: parameter
// types (hit by parseParam) and non-envelope return types (hit by the
// response wrapper).
func (s *Server) pretouchHandlerTypes(handlerT reflect.Type, needsContext bool) {
	for i := range handlerT.NumIn() {
		if i == 0 && needsContext {
			continue
		}
		s.pretouch(handlerT.In(i))
	}
	for out := range handlerT.Outs() {
		if out == errorType || out == headerType {
			continue
		}
		s.pretouch(out)
	}
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

	dec := jsonx.NewDecoder(bufferedReader)
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

	result, err := jsonx.Marshal(resp)
	return result, header, err
}

func (s *Server) handleBatchRequest(ctx context.Context, batchReq []json.RawMessage) ([]byte, http.Header, error) {
	var (
		mutex     sync.Mutex
		responses []json.RawMessage
		headers   []http.Header
	)

	addResponse := func(response any, header http.Header) {
		if responseJSON, err := jsonx.Marshal(response); err != nil {
			s.logger.Error("failed to marshal response", zap.Error(err))
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
		reqDec := jsonx.NewDecoder(bytes.NewBuffer(rawReq))
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

	result, err := jsonx.Marshal(responses)

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

// TODO: add recover() to catch panics from handlers/validators and return a JSON-RPC internal error
// instead of crashing the HTTP connection
func (s *Server) handleRequest(ctx context.Context, req *Request) (*response, http.Header, error) {
	s.logger.Trace("Received request", zap.Object("req", req))

	header := http.Header{}
	if err := req.isSane(); err != nil {
		s.logger.Trace("Request sanity check failed", zap.Error(err))
		return nil, header, err
	}

	res := &response{
		Version: "2.0",
		ID:      req.ID,
	}

	calledMethod, found := s.methods[req.Method]
	if !found {
		res.Error = Err(MethodNotFound, nil)
		s.logger.Trace(
			"Method not found in request",
			zap.String("method", log.SanitizeString(req.Method)),
		)
		return res, header, nil
	}

	handlerTimer := time.Now()
	s.listener.OnNewRequest(req.Method)
	args, err := s.buildArguments(ctx, req.Params, &calledMethod)
	if err != nil {
		res.Error = Err(InvalidParams, err.Error())
		s.logger.Trace("Error building arguments for RPC call", zap.Error(err))
		return res, header, nil
	}
	defer func() {
		s.listener.OnRequestHandled(req.Method, time.Since(handlerTimer))
	}()

	tuple := calledMethod.handlerVal.Call(args)
	if res.ID == nil { // notification
		s.logger.Trace("Notification received, no response expected")
		return nil, header, nil
	}

	errorIndex := 1
	if len(tuple) == 3 {
		errorIndex = 2
		header = (tuple[1].Interface()).(http.Header)
	}

	if !tuple[errorIndex].IsNil() {
		res.Error = tuple[errorIndex].Interface().(*Error)
		if res.Error.Code == InternalError {
			s.listener.OnRequestFailed(req.Method, res.Error)
			reqJSON, _ := jsonx.Marshal(req)
			errJSON, _ := jsonx.Marshal(res.Error)
			s.logger.Debug("Failed handing RPC request",
				zap.String("req", log.SanitizeString(string(reqJSON))),
				zap.String("res", log.SanitizeString(string(errJSON))),
			)
		}
		return res, header, nil
	}
	res.Result = tuple[0].Interface()

	return res, header, nil
}

// buildArguments materializes []reflect.Value for the handler call from
// the request's raw params. The hot path reads only cached fields on
// Method (inTypes, paramByName, handlerVal); no reflect.TypeOf /
// NumIn / In is called per request, and per-slot decode is a single
// jsonx.UnmarshalString — no Marshal→Unmarshal round-trip.
func (s *Server) buildArguments(ctx context.Context, params json.RawMessage, method *Method) ([]reflect.Value, error) {
	numNonCtx := len(method.inTypes)
	addContext := 0
	if method.needsContext {
		addContext = 1
	}
	args := make([]reflect.Value, 0, numNonCtx+addContext)
	if method.needsContext {
		args = append(args, reflect.ValueOf(ctx))
	}

	switch paramsKind(params) {
	case paramsKindNone:
		if method.requiredParamCount > 0 {
			return nil, errors.New("missing non-optional param field")
		}
		for i := range numNonCtx {
			args = append(args, method.inZeroes[i])
		}
		return args, nil
	case paramsKindArray:
		var node ast.Node
		if err := node.UnmarshalJSON(params); err != nil {
			return nil, err
		}
		return s.buildPositionalArgs(&node, method, args)
	case paramsKindObject:
		var node ast.Node
		if err := node.UnmarshalJSON(params); err != nil {
			return nil, err
		}
		return s.buildNamedArgs(&node, method, args)
	default:
		// Unreachable: isSane already rejects non-container params.
		return nil, errors.New("impossible param type: check request.isSane")
	}
}

func (s *Server) buildPositionalArgs(
	node *ast.Node, method *Method, args []reflect.Value,
) ([]reflect.Value, error) {
	iter, err := node.Values()
	if err != nil {
		return nil, err
	}
	consumed := 0
	var elem ast.Node
	for iter.Next(&elem) {
		if consumed >= len(method.inTypes) {
			return nil, errors.New("missing/unexpected params in list")
		}
		raw, rawErr := elem.Raw()
		if rawErr != nil {
			return nil, rawErr
		}
		v, decErr := s.decodeIntoSlot(raw, method, consumed)
		if decErr != nil {
			return nil, decErr
		}
		args = append(args, v)
		consumed++
	}
	if consumed < method.requiredParamCount {
		return nil, errors.New("missing/unexpected params in list")
	}
	for i := consumed; i < len(method.inTypes); i++ {
		args = append(args, method.inZeroes[i])
	}
	return args, nil
}

func (s *Server) buildNamedArgs(
	node *ast.Node, method *Method, args []reflect.Value,
) ([]reflect.Value, error) {
	iter, err := node.Properties()
	if err != nil {
		return nil, err
	}

	rawByIndex := make([]string, len(method.inTypes))
	found := make([]bool, len(method.inTypes))
	var unknown []string

	var pair ast.Pair
	for iter.Next(&pair) {
		idx, ok := method.paramByName[pair.Key]
		if !ok {
			unknown = append(unknown, pair.Key)
			continue
		}
		raw, rawErr := pair.Value.Raw()
		if rawErr != nil {
			return nil, rawErr
		}
		rawByIndex[idx] = raw
		found[idx] = true
	}

	// Preserve the original error precedence: missing-required wins over
	// unexpected-extras when both are present.
	for i, param := range method.Params {
		if !found[i] && !param.Optional {
			return nil, errors.New("missing non-optional param: " + param.Name)
		}
	}
	if len(unknown) > 0 {
		return nil, errors.New("unexpected params: " + strings.Join(unknown, ", "))
	}

	for i := range method.inTypes {
		if found[i] {
			v, decErr := s.decodeIntoSlot(rawByIndex[i], method, i)
			if decErr != nil {
				return nil, decErr
			}
			args = append(args, v)
		} else {
			args = append(args, method.inZeroes[i])
		}
	}
	return args, nil
}

// decodeIntoSlot unmarshals raw JSON directly into a freshly-allocated
// value of the i-th declared param's type (single sonic decode pass —
// no Marshal first). validateParam is skipped for types whose registry
// scan found no `validate:` tags transitively.
func (s *Server) decodeIntoSlot(raw string, method *Method, i int) (reflect.Value, error) {
	handlerParam := reflect.New(method.inTypes[i])
	if err := jsonx.UnmarshalString(raw, handlerParam.Interface()); err != nil {
		return reflect.Value{}, err
	}
	elem := handlerParam.Elem()
	if s.validator != nil && method.needsValidation[i] {
		if err := s.validateParam(elem); err != nil {
			return reflect.Value{}, err
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

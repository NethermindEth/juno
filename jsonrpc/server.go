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

	bufferSize = 128
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

// Dispatch is the type-erased per-request handler installed by the
// Register methods. It owns parse-validate-call: param decode targets
// stack-allocated typed locals owned by the adapter closure, and the
// user handler is called via a typed function value.
type Dispatch func(
	ctx context.Context,
	v Validator,
	params json.RawMessage,
) (result any, header http.Header, rpcErr *Error)

type Method struct {
	Name string

	// Dispatch is the typed closure built by Register. It owns the per-request parse, validate,
	// and handler-call pipeline. Populated by the public constructors in register.go.
	Dispatch Dispatch

	// ParamStructType is the reflect.Type of the P type parameter used
	// when the method was registered. Surfaced for tests that need to
	// walk param shapes (e.g. cross-version validator compatibility).
	ParamStructType reflect.Type

	// pretouchTypes lists the param-struct + result reflect.Types
	// the constructor wants sonic to pre-compile encode/decode paths
	// for. Read once at registration.
	pretouchTypes []reflect.Type
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

// RegisterMethods adds each Method's Dispatch closure to the server's
// routing table and pre-compiles the param/result types via sonic.
// Methods are produced by Register / RegisterC / RegisterH / RegisterCH.
func (s *Server) RegisterMethods(methods ...Method) error {
	for idx := range methods {
		if err := s.registerMethod(methods[idx]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) registerMethod(method Method) error {
	if method.Dispatch == nil {
		return errors.New("jsonrpc: method has no Dispatch; use Register/RegisterC/RegisterH/RegisterCH")
	}
	for _, t := range method.pretouchTypes {
		s.pretouch(t)
	}
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
	defer func() {
		s.listener.OnRequestHandled(req.Method, time.Since(handlerTimer))
	}()

	result, dispatchHeader, rpcErr := calledMethod.Dispatch(ctx, s.validator, req.Params)
	if res.ID == nil { // notification
		s.logger.Trace("Notification received, no response expected")
		return nil, header, nil
	}
	if dispatchHeader != nil {
		header = dispatchHeader
	}
	if rpcErr != nil {
		res.Error = rpcErr
		if rpcErr.Code == InternalError {
			s.listener.OnRequestFailed(req.Method, rpcErr)
			reqJSON, _ := jsonx.Marshal(req)
			errJSON, _ := jsonx.Marshal(rpcErr)
			s.logger.Debug(
				"Failed handing RPC request",
				zap.String("req", log.SanitizeString(string(reqJSON))),
				zap.String("res", log.SanitizeString(string(errJSON))),
			)
		}
		return res, header, nil
	}
	res.Result = result
	return res, header, nil
}

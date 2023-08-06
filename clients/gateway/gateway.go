package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

var (
	InvalidContractClass            ErrorCode = "StarknetErrorCode.INVALID_CONTRACT_CLASS"
	UndeclaredClass                 ErrorCode = "StarknetErrorCode.UNDECLARED_CLASS"
	ClassAlreadyDeclared            ErrorCode = "StarknetErrorCode.CLASS_ALREADY_DECLARED"
	InsufficientMaxFee              ErrorCode = "StarknetErrorCode.INSUFFICIENT_MAX_FEE"
	InsufficientAccountBalance      ErrorCode = "StarknetErrorCode.INSUFFICIENT_ACCOUNT_BALANCE"
	ValidateFailure                 ErrorCode = "StarknetErrorCode.VALIDATE_FAILURE"
	ContractBytecodeSizeTooLarge    ErrorCode = "StarknetErrorCode.CONTRACT_BYTECODE_SIZE_TOO_LARGE"
	DuplicatedTransaction           ErrorCode = "StarknetErrorCode.DUPLICATED_TRANSACTION"
	InvalidTransactionNonce         ErrorCode = "StarknetErrorCode.INVALID_TRANSACTION_NONCE"
	CompilationFailed               ErrorCode = "StarknetErrorCode.COMPILATION_FAILED"
	InvalidCompiledClassHash        ErrorCode = "StarknetErrorCode.INVALID_COMPILED_CLASS_HASH"
	ContractClassObjectSizeTooLarge ErrorCode = "StarknetErrorCode.CONTRACT_CLASS_OBJECT_SIZE_TOO_LARGE"
	InvalidTransactionVersion       ErrorCode = "StarknetErrorCode.INVALID_TRANSACTION_VERSION"
	InvalidContractClassVersion     ErrorCode = "StarknetErrorCode.INVALID_CONTRACT_CLASS_VERSION"
)

type Client struct {
	url     string
	client  *http.Client
	timeout time.Duration
	log     utils.SimpleLogger
}

// NewTestClient returns a client and a function to close a test server.
func NewTestClient(t *testing.T) *Client {
	srv := newTestServer()
	t.Cleanup(srv.Close)

	return NewClient(srv.URL, utils.NewNopZapLogger())
}

func newTestServer() *httptest.Server {
	// As this is a test sever we are mimic response for one good and one bad request.
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write([]byte(err.Error()))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		// empty request: "{}"
		emptyReqLen := 4
		if string(b) == "null" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if len(b) <= emptyReqLen {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write([]byte(`{"code": "Malformed Request", "message": "empty request"}`))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		hash := new(felt.Felt).SetBytes([]byte("random"))
		resp := fmt.Sprintf("{\"code\": \"TRANSACTION_RECEIVED\", \"transaction_hash\": %q, \"address\": %q}", hash.String(), hash.String())
		_, err = w.Write([]byte(resp))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

func NewClient(gatewayURL string, log utils.SimpleLogger) *Client {
	gatewayURL = strings.TrimSuffix(gatewayURL, "/")
	return &Client{
		url:     gatewayURL,
		timeout: 10 * time.Second,
		client:  http.DefaultClient,
		log:     log,
	}
}

func (c *Client) AddTransaction(txn json.RawMessage) (json.RawMessage, error) {
	return c.post(c.url+"/add_transaction", txn)
}

// post performs additional utility function over doPost method
func (c *Client) post(url string, data any) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	resp, err := c.doPost(ctx, url, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var gatewayError Error
		body, readErr := io.ReadAll(resp.Body)
		if readErr == nil && len(body) > 0 {
			if err := json.Unmarshal(body, &gatewayError); err == nil {
				if len(gatewayError.Code) != 0 {
					return nil, &gatewayError
				}
			}
			return nil, errors.New(string(body))
		}
		return nil, errors.New(resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// doPost performs a "POST" http request with the given URL and a JSON payload derived from the provided data
// it returns response without additional error handling
func (c *Client) doPost(ctx context.Context, url string, data any) (*http.Response, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Juno v0.5.0-rc0")
	return c.client.Do(req)
}

type ErrorCode string

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e Error) Error() string {
	return e.Message
}

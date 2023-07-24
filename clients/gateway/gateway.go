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
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type ErrorCode string

var (
	BlockNotFound                   ErrorCode = "StarknetErrorCode.BLOCK_NOT_FOUND"
	EntryPointNotFound              ErrorCode = "StarknetErrorCode.ENTRY_POINT_NOT_FOUND_IN_CONTRACT"
	OutOfRangeContractAddress       ErrorCode = "StarknetErrorCode.OUT_OF_RANGE_CONTRACT_ADDRESS"
	SchemaValidationError           ErrorCode = "StarknetErrorCode.SCHEMA_VALIDATION_ERROR"
	TransactionFailed               ErrorCode = "StarknetErrorCode.TRANSACTION_FAILED"
	UninitializedContract           ErrorCode = "StarknetErrorCode.UNINITIALIZED_CONTRACT"
	OutOfRangeBlockHash             ErrorCode = "StarknetErrorCode.OUT_OF_RANGE_BLOCK_HASH"
	OutOfRangeTransactionHash       ErrorCode = "StarknetErrorCode.OUT_OF_RANGE_TRANSACTION_HASH"
	MalformedRequest                ErrorCode = "StarknetErrorCode.MALFORMED_REQUEST"
	UnsupportedSelectorForFee       ErrorCode = "StarknetErrorCode.UNSUPPORTED_SELECTOR_FOR_FEE"
	InvalidContractDefinition       ErrorCode = "StarknetErrorCode.INVALID_CONTRACT_DEFINITION"
	NotPermittedContract            ErrorCode = "StarknetErrorCode.NON_PERMITTED_CONTRACT"
	UndeclaredClass                 ErrorCode = "StarknetErrorCode.UNDECLARED_CLASS"
	TransactionLimitExceeded        ErrorCode = "StarknetErrorCode.TRANSACTION_LIMIT_EXCEEDED"
	InvalidTransactionNonce         ErrorCode = "StarknetErrorCode.INVALID_TRANSACTION_NONCE"
	OutOfRangeFee                   ErrorCode = "StarknetErrorCode.OUT_OF_RANGE_FEE"
	InvalidTransactionVersion       ErrorCode = "StarknetErrorCode.INVALID_TRANSACTION_VERSION"
	InvalidProgram                  ErrorCode = "StarknetErrorCode.INVALID_PROGRAM"
	DeprecatedTransaction           ErrorCode = "StarknetErrorCode.DEPRECATED_TRANSACTION"
	InvalidCompiledClassHash        ErrorCode = "StarknetErrorCode.INVALID_COMPILED_CLASS_HASH"
	CompilationFailed               ErrorCode = "StarknetErrorCode.COMPILATION_FAILED"
	UnauthorizedEntryPointForInvoke ErrorCode = "StarknetErrorCode.UNAUTHORIZED_ENTRY_POINT_FOR_INVOKE"
	InvalidContractClass            ErrorCode = "StarknetErrorCode.INVALID_CONTRACT_CLASS"
	ClassHashNotFound               ErrorCode = "StarknetErrorCode.CLASS_HASH_NOT_FOUND"
	ClassAlreadyDeclared            ErrorCode = "StarknetErrorCode.CLASS_ALREADY_DECLARED"
	InsufficientMaxFee              ErrorCode = "StarknetErrorCode.INSUFFICIENT_MAX_FEE"
	InsufficientAccountBalance      ErrorCode = "StarknetErrorCode.INSUFFICIENT_ACCOUNT_BALANCE"
	ValidationFailure               ErrorCode = "StarknetErrorCode.VALIDATION_FAILURE"
	ContractBytecodeSizeTooLarge    ErrorCode = "StarknetErrorCode.CONTRACT_BYTECODE_SIZE_TOO_LARGE"
	NonAccount                      ErrorCode = "StarknetErrorCode.NON_ACCOUNT"
	DuplicateTx                     ErrorCode = "StarknetErrorCode.DUPLICATE_TX"
)

type Client struct {
	url     string
	client  *http.Client
	timeout time.Duration
	log     utils.SimpleLogger
}

type (
	closeTestClient func()
)

// NewTestClient returns a client and a function to close a test server.
func NewTestClient() (*Client, closeTestClient) {
	srv := newTestServer()
	return NewClient(srv.URL, utils.NewNopZapLogger()), srv.Close
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
		if len(b) <= emptyReqLen {
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
	endpoint := c.url + "/add_transaction"

	body, err := c.post(endpoint, txn)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	res, readErr := io.ReadAll(body)
	if readErr != nil {
		return nil, readErr
	}

	return res, nil
}

// post performs additional utility function over doPost method
func (c *Client) post(url string, data any) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	resp, err := c.doPost(ctx, url, data)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		err = c.tryDecodeErr(resp.Body)
		// decoding failed, at least pass info about http code
		if err == nil {
			err = fmt.Errorf("received non 200 status code: %d", resp.StatusCode)
		}
		return nil, err
	}

	return resp.Body, nil
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
	return c.client.Do(req)
}

func (c *Client) tryDecodeErr(resp io.Reader) error {
	var gatewayError struct {
		Code    ErrorCode `json:"code"`
		Message string    `json:"message"`
	}

	if err := json.NewDecoder(resp).Decode(&gatewayError); err != nil {
		c.log.Errorw("failed to decode gateway error", "err", err)
	}

	var err error
	if gatewayError.Message != "" {
		err = errors.New(gatewayError.Message)
	}
	return err
}

package gateway

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NethermindEth/juno/utils"
)

var (
	InvalidContractClass              ErrorCode = "StarknetErrorCode.INVALID_CONTRACT_CLASS"
	UndeclaredClass                   ErrorCode = "StarknetErrorCode.UNDECLARED_CLASS"
	ClassAlreadyDeclared              ErrorCode = "StarknetErrorCode.CLASS_ALREADY_DECLARED"
	InsufficientMaxFee                ErrorCode = "StarknetErrorCode.INSUFFICIENT_MAX_FEE"
	InsufficientResourcesForValidate  ErrorCode = "StarknetErrorCode.INSUFFICIENT_RESOURCES_FOR_VALIDATE"
	InsufficientAccountBalance        ErrorCode = "StarknetErrorCode.INSUFFICIENT_ACCOUNT_BALANCE"
	ValidateFailure                   ErrorCode = "StarknetErrorCode.VALIDATE_FAILURE"
	ContractBytecodeSizeTooLarge      ErrorCode = "StarknetErrorCode.CONTRACT_BYTECODE_SIZE_TOO_LARGE"
	DuplicatedTransaction             ErrorCode = "StarknetErrorCode.DUPLICATED_TRANSACTION"
	InvalidTransactionNonce           ErrorCode = "StarknetErrorCode.INVALID_TRANSACTION_NONCE"
	CompilationFailed                 ErrorCode = "StarknetErrorCode.COMPILATION_FAILED"
	InvalidCompiledClassHash          ErrorCode = "StarknetErrorCode.INVALID_COMPILED_CLASS_HASH"
	ContractClassObjectSizeTooLarge   ErrorCode = "StarknetErrorCode.CONTRACT_CLASS_OBJECT_SIZE_TOO_LARGE"
	InvalidTransactionVersion         ErrorCode = "StarknetErrorCode.INVALID_TRANSACTION_VERSION"
	InvalidContractClassVersion       ErrorCode = "StarknetErrorCode.INVALID_CONTRACT_CLASS_VERSION"
	FeeBelowMinimum                   ErrorCode = "StarknetErrorCode.FEE_BELOW_MINIMUM"
	ReplacementTransactionUnderPriced ErrorCode = "StarknetErrorCode.REPLACEMENT_TRANSACTION_UNDERPRICED"
	InvalidProof                      ErrorCode = "StarknetErrorCode.INVALID_PROOF"
)

// Payload size threshold for Gzip compression. 1KB
const gzipMinSize = 1024

type Client struct {
	url       string
	client    *http.Client
	listener  EventListener
	log       utils.StructuredLogger
	userAgent string
	apiKey    string
}

func (c *Client) WithUserAgent(ua string) *Client {
	c.userAgent = ua
	return c
}

func (c *Client) WithAPIKey(key string) *Client {
	c.apiKey = key
	return c
}

func (c *Client) WithListener(l EventListener) *Client {
	c.listener = l
	return c
}

func NewClient(gatewayURL string, log utils.StructuredLogger) *Client {
	gatewayURL = strings.TrimSuffix(gatewayURL, "/")
	return &Client{
		url: gatewayURL,
		client: &http.Client{
			Timeout: time.Minute,
		},
		listener: &SelectiveListener{},
		log:      log,
	}
}

func (c *Client) AddTransaction(ctx context.Context, txn json.RawMessage) (json.RawMessage, error) {
	return c.post(ctx, c.url+"/add_transaction", txn)
}

// post performs additional utility function over doPost method
func (c *Client) post(ctx context.Context, url string, data any) ([]byte, error) {
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
	jsonBody, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	bodyReader, compressed, err := prepareRequestBody(jsonBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.apiKey != "" {
		req.Header.Set("X-Throttling-Bypass", c.apiKey)
	}
	reqTimer := time.Now()
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	c.listener.OnResponse(req.URL.Path, resp.StatusCode, time.Since(reqTimer))
	return resp, nil
}

func prepareRequestBody(jsonBody []byte) (io.Reader, bool, error) {
	if len(jsonBody) <= gzipMinSize {
		return bytes.NewReader(jsonBody), false, nil
	}

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	if _, err := gzWriter.Write(jsonBody); err != nil {
		return nil, false, fmt.Errorf("writing gzip content: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return nil, false, fmt.Errorf("closing gzip writer: %w", err)
	}

	return &buf, true, nil
}

type ErrorCode string

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e Error) Error() string {
	return e.Message
}

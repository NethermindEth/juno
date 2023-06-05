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

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"

	"github.com/go-playground/validator/v10"
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
func NewTestClient(network utils.Network) (*Client, closeTestClient) {
	srv := newTestServer(network)
	return NewClient(srv.URL, utils.NewNopZapLogger()), srv.Close
}

func newTestServer(network utils.Network) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invokeTx := new(BroadcastedInvokeTxn)
		err := json.NewDecoder(r.Body).Decode(&invokeTx)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write([]byte(err.Error()))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		if err = checkAddInvokeTx(invokeTx); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write([]byte(err.Error()))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		txHash := crypto.PedersenArray(
			new(felt.Felt).SetBytes([]byte("invoke")),
			new(felt.Felt).SetBytes([]byte(invokeTx.Version)),
			invokeTx.SenderAddress,
			new(felt.Felt),
			crypto.PedersenArray(invokeTx.Calldata...),
			invokeTx.MaxFee,
			network.ChainID(),
			invokeTx.Nonce,
		)
		resp := fmt.Sprintf("{\"code\": \"TRANSACTION_RECEIVED\", \"transaction_hash\": %q}", txHash.String())
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

// checkAddInvokeTx checks invoke-transactions for validation and version errors, etc.
func checkAddInvokeTx(txn *BroadcastedInvokeTxn) error {
	validate := validator.New()
	if err := validate.Struct(txn); err != nil {
		errMsg := "{\"message\": \"{%s: ['Missing data for required field.']}\"}"
		return fmt.Errorf(errMsg, err.(validator.ValidationErrors)[0].Field())
	}

	if txn.Version != "0x1" {
		errMsg := "{\"message\": \"Transaction version %s is not supported. Supported versions: [1].\"}"
		return fmt.Errorf(errMsg, txn.Version)
	}

	if txn.MaxFee.ShortString() == "0x0" {
		return fmt.Errorf("{\"message\": \"max_fee must be bigger than 0.\\n0 >= 0\"}")
	}

	return nil
}

func (c *Client) AddInvokeTransaction(ctx context.Context, txn *json.RawMessage) (*core.InvokeTransaction, error) {
	endpoint := c.url + "/add_transaction"

	body, err := c.post(ctx, endpoint, txn)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var resp core.InvokeTransaction
	if err = json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// post performs additional utility function over doPost method
func (c *Client) post(ctx context.Context, url string, data any) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
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
		Code    string `json:"code"`
		Message string `json:"message"`
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

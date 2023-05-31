package gateway

import (
	"bytes"
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

//go:generate mockgen -destination=../../mocks/mock_gateway.go -package=mocks github.com/NethermindEth/juno/clients/gateway Gateway
type Gateway interface {
	AddInvokeTransaction(context.Context, *BroadcastedInvokeTxn) (*InvokeTxResponse, error)
}

type Client struct {
	url     string
	client  *http.Client
	timeout time.Duration
	log     utils.SimpleLogger
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

func (c *Client) AddInvokeTransaction(ctx context.Context, txn *BroadcastedInvokeTxn) (*InvokeTxResponse, error) {
	endpoint := c.url + "/add_transaction"

	body, err := c.post(ctx, endpoint, txn)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var resp InvokeTxResponse
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

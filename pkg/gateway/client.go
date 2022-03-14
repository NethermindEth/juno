// Package gateway represent a client for the Feeder Gateway Connection.
// For more details of the implementation, see this client https://github.com/starkware-libs/cairo-lang/blob/master/src/starkware/starknet/services/api/feeder_gateway/feeder_gateway_client.py
package gateway

import (
	"bytes"
	"encoding/json"
	"github.com/NethermindEth/juno/internal/errpkg"
	"github.com/NethermindEth/juno/internal/log"
	"io"
	"net/http"
	"net/url"
	"time"
)

const badBaseUrl = "Bad base url"

// Client A client for the StarkNet FeederGateway
type Client struct {
	BaseURL   *url.URL
	UserAgent string

	httpClient *http.Client
}

// NewClient returns a new Client.
func NewClient(baseUrl string) *Client {
	u, err := url.Parse(baseUrl)
	errpkg.CheckFatal(err, badBaseUrl)
	return &Client{
		BaseURL: u,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// newRequest creates a new request based on params, in any other case returns an error.
func (c *Client) newRequest(method, path string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.BaseURL.ResolveReference(rel)
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	return req, nil
}

// do execute a request and waits for response, in any other case returns an error.
func (c *Client) do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Default.With("Error", err, "Request Path", req.URL.RawPath).Error("Error closing body of response")
		}
	}(resp.Body)
	err = json.NewDecoder(resp.Body).Decode(v)
	return resp, err
}

// getContractAddresses creates a new request to get Contract Addresses from the Getaway
func (c Client) getContractAddresses() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_contract_addresses", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// callContract creates a new request to get Contract Addresses from the Getaway
func (c Client) callContract() (map[string]string, error) {
	req, err := c.newRequest("GET", "/call_contract", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getBlock creates a new request to get Contract Addresses from the Getaway
func (c Client) getBlock() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_block", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getStateUpdate creates a new request to get Contract Addresses from the Getaway
func (c Client) getStateUpdate() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_state_update", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getCode creates a new request to get Contract Addresses from the Getaway
func (c Client) getCode() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_code", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getFullContract creates a new request to get Contract Addresses from the Getaway
func (c Client) getFullContract() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_full_contract", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getFullContract creates a new request to get Contract Addresses from the Getaway
func (c Client) getStorageAt() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_storage_at", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getTransactionStatus creates a new request to get Contract Addresses from the Getaway
func (c Client) getTransactionStatus() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_transaction_status", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getTransaction creates a new request to get Contract Addresses from the Getaway
func (c Client) getTransaction() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_transaction", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getTransactionReceipt creates a new request to get Contract Addresses from the Getaway
func (c Client) getTransactionReceipt() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_transaction_receipt", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getBlockHashById creates a new request to get Contract Addresses from the Getaway
func (c Client) getBlockHashById() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_block_hash_by_id", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getBlockIdByHash creates a new request to get Contract Addresses from the Getaway
func (c Client) getBlockIdByHash() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_block_id_by_hash", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getTransactionHashById creates a new request to get Contract Addresses from the Getaway
func (c Client) getTransactionHashById() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_transaction_hash_by_id", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// getTransactionIdByHash creates a new request to get Contract Addresses from the Getaway
func (c Client) getTransactionIdByHash() (map[string]string, error) {
	req, err := c.newRequest("GET", "/get_transaction_id_by_hash", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

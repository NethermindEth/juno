// Package feeder_gateway represent a client for the Feeder Gateway Connection.
// For more details of the implementation, see this client https://github.com/starkware-libs/cairo-lang/blob/master/src/starkware/starknet/services/api/feeder_gateway/feeder_gateway_client.py
package feeder_gateway

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

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . FeederHttpClient
type FeederHttpClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Client A client for the StarkNet FeederGateway
type Client struct {
	BaseURL   *url.URL
	UserAgent string
	BaseApi   string

	httpClient *FeederHttpClient
}

// NewClient returns a new Client.
func NewClient(baseUrl, baseApi string, client *FeederHttpClient) *Client {
	u, err := url.Parse(baseUrl)
	errpkg.CheckFatal(err, badBaseUrl)
	if client == nil {
		var p FeederHttpClient
		c := http.Client{
			Timeout: 50 * time.Second,
		}
		p = &c
		client = &p
	}
	return &Client{
		BaseURL:    u,
		BaseApi:    baseApi,
		httpClient: client,
	}
}

func formattedBlockIdentifier(blockHash, blockNumber string) map[string]string {
	if len(blockHash) == 0 && len(blockNumber) == 0 {
		return nil
	}
	if len(blockHash) == 0 {
		return map[string]string{"blockNumber": blockNumber}
	}
	return map[string]string{"blockHash": blockHash}
}

func TxnIdentifier(txHash, txId string) map[string]string {
	if len(txHash) == 0 && len(txId) == 0 {
		return nil
	}

	if len(txHash) == 0 {
		return map[string]string{"transactionId": txId}
	}
	return map[string]string{"transactionHash": txHash}

}

// newRequest creates a new request based on params, in any other case returns an error.
func (c *Client) newRequest(method, path string, query map[string]string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: c.BaseApi + path}
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
	log.Default.With("Request Url", req.URL).Info("Making a request")
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if query != nil {
		q := req.URL.Query()
		for k, v := range query {
			q.Add(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	return req, nil
}

// do execute a request and waits for response, in any other case returns an error.
func (c *Client) do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := (*c.httpClient).Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			// notest
			log.Default.With("Error", err).Error("Error closing body of response")
			return
		}
	}(resp.Body)
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Default.With("Error", err).Debug("Error response of the request")
		return nil, err
	}
	err = json.Unmarshal(b, v)
	return resp, err
}

// GetContractAddresses creates a new request to get Contract Addresses from the Getaway
func (c Client) GetContractAddresses() (ContractAddresses, error) {
	log.Default.With("Gateway Url", c.BaseURL).Info("Getting Contract Address from Gateway")
	req, err := c.newRequest("GET", "/get_contract_addresses", nil, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return ContractAddresses{}, err
	}
	var response ContractAddresses
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return ContractAddresses{}, err
	}
	return response, err
}

// CallContract creates a new request to call a contract in the gateway
func (c Client) CallContract(invokeFunction InvokeFunction, blockHash, blockNumber string) (map[string][]string, error) {
	req, err := c.newRequest("POST", "/call_contract", formattedBlockIdentifier(blockHash, blockNumber), invokeFunction)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response map[string][]string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// GetBlock creates a new request to get a block of the Gateway
func (c Client) GetBlock(blockHash, blockNumber string) (StarknetBlock, error) {
	req, err := c.newRequest("GET", "/get_block", formattedBlockIdentifier(blockHash, blockNumber), nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return StarknetBlock{}, err
	}
	var response StarknetBlock
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return StarknetBlock{}, err
	}
	return response, err
}

// GetStateUpdate creates a new request to get Contract Addresses from the Getaway
func (c Client) GetStateUpdate(blockHash, blockNumber string) (StateUpdateResponse, error) {
	req, err := c.newRequest("GET", "/get_state_update", formattedBlockIdentifier(blockHash, blockNumber), nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return StateUpdateResponse{}, err
	}

	var response StateUpdateResponse
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL, "Request", req.URL.RawPath).
			Error("Error connecting to getaway.")
		return StateUpdateResponse{}, err
	}
	return response, err
}

// GetCode creates a new request to get Code of Contract address
func (c Client) GetCode(contractAddress, blockHash, blockNumber string) ([]string, error) {
	blockIdentifier := formattedBlockIdentifier(blockHash, blockNumber)
	if blockIdentifier == nil {
		blockIdentifier = map[string]string{}
	}
	blockIdentifier["contractAddress"] = contractAddress
	req, err := c.newRequest("GET", "/get_code",
		blockIdentifier, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response []string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// GetFullContract creates a new request to get thw full state of a Contract
func (c Client) GetFullContract(contractAddress, blockHash, blockNumber string) (interface{}, error) {
	blockIdentifier := formattedBlockIdentifier(blockHash, blockNumber)
	if blockIdentifier == nil {
		blockIdentifier = map[string]string{}
	}
	blockIdentifier["contractAddress"] = contractAddress

	req, err := c.newRequest("GET", "/get_full_contract",
		blockIdentifier, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response interface{}
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// GetStorageAt creates a new request to get Storage of Contract.
func (c Client) GetStorageAt(contractAddress, key, blockHash, blockNumber string) (string, error) {
	blockIdentifier := formattedBlockIdentifier(blockHash, blockNumber)
	if blockIdentifier == nil {
		blockIdentifier = map[string]string{}
	}
	blockIdentifier["contractAddress"] = contractAddress
	blockIdentifier["key"] = key

	req, err := c.newRequest("GET", "/get_storage_at",
		blockIdentifier, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return "", err
	}
	var response string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return "", err
	}
	return response, err
}

// GetTransactionStatus creates a new request to get a transaction Status
func (c Client) GetTransactionStatus(txHash, txId string) (interface{}, error) {
	req, err := c.newRequest("GET", "/get_transaction_status", TxnIdentifier(txHash, txId), nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var response interface{}
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return response, err
}

// GetTransaction creates a new request to get a TransactionInfo
func (c Client) GetTransaction(txHash, txId string) (TransactionInfo, error) {
	req, err := c.newRequest("GET", "/get_transaction", TxnIdentifier(txHash, txId), nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return TransactionInfo{}, err
	}
	var response TransactionInfo
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return TransactionInfo{}, err
	}
	return response, err
}

// GetTransactionReceipt creates a new request to get a TransactionReceipt
func (c Client) GetTransactionReceipt(txHash, txId string) (TransactionReceipt, error) {
	req, err := c.newRequest("GET", "/get_transaction_receipt", TxnIdentifier(txHash, txId), nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return TransactionReceipt{}, err
	}
	var response TransactionReceipt
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return TransactionReceipt{}, err
	}
	return response, err
}

// GetBlockHashById creates a new request to get block Hash based on block ID
func (c Client) GetBlockHashById(blockId string) (string, error) {
	req, err := c.newRequest("GET", "/get_block_hash_by_id", map[string]string{"blockId": blockId}, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return "", err
	}
	var response string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return "", err
	}
	return response, err
}

// GetBlockIdByHash creates a new request to get Block ID based on Block Hash
func (c Client) GetBlockIdByHash(blockHash string) (string, error) {
	req, err := c.newRequest("GET", "/get_block_id_by_hash",
		map[string]string{"blockHash": blockHash}, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return "", err
	}
	var response string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return "", err
	}
	return response, err
}

// GetTransactionHashById creates a new request to get a Transaction hash based on Transaction ID
func (c Client) GetTransactionHashById(txId string) (string, error) {
	req, err := c.newRequest("GET", "/get_transaction_hash_by_id",
		map[string]string{"transactionId": txId}, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return "", err
	}
	var response string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return "", err
	}
	return response, err
}

// GetTransactionIdByHash creates a new request to get a Transaction ID based on Transaction Hash
func (c Client) GetTransactionIdByHash(txHash string) (string, error) {
	req, err := c.newRequest("GET", "/get_transaction_id_by_hash",
		map[string]string{"transactionHash": txHash}, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return "", err
	}
	var response string
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return "", err
	}
	return response, err
}

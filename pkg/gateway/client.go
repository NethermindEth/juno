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
	"strconv"
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

func getFormattedBlockIdentifier(blockHash, blockNumber string) string {
	if len(blockHash) == 0 {
		return "blockNumber=" + blockNumber
	}
	return "blockHash=" + blockHash
}

func TxnIdentifier(txHash, txId string) string {
	if len(txHash) == 0 {
		return "transactionId=" + txId
	}
	return "transactionHash=" + txHash

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

// callContract creates a new request to call a contract in the gateway
func (c Client) callContract(invokeFunction InvokeFunction, blockHash, blockNumber string) (map[string][]string, error) {
	req, err := c.newRequest("POST", "/call_contract?"+getFormattedBlockIdentifier(blockHash, blockNumber), invokeFunction)
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

// getBlock creates a new request to get a block of the Gateway
func (c Client) getBlock(blockHash, blockNumber string) (StarknetBlock, error) {
	req, err := c.newRequest("GET", "/get_block?"+getFormattedBlockIdentifier(blockHash, blockNumber), nil)
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

// getStateUpdate creates a new request to get Contract Addresses from the Getaway
func (c Client) getStateUpdate(blockHash, blockNumber string) (interface{}, error) {
	req, err := c.newRequest("GET", "/get_state_update?"+getFormattedBlockIdentifier(blockHash, blockNumber), nil)
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

// getCode creates a new request to get Code of Contract address
func (c Client) getCode(contractAddress, blockHash, blockNumber string) ([]string, error) {
	req, err := c.newRequest("GET", "/get_code?contractAddress="+contractAddress+"&"+
		getFormattedBlockIdentifier(blockHash, blockNumber), nil)
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

// getFullContract creates a new request to get thw full state of a Contract
func (c Client) getFullContract(contractAddress, blockHash, blockNumber string) (interface{}, error) {
	req, err := c.newRequest("GET", "/get_full_contract?contractAddress="+contractAddress+"&"+
		getFormattedBlockIdentifier(blockHash, blockNumber), nil)
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

// getStorageAt creates a new request to get Storage of address.
func (c Client) getStorageAt(contractAddress, key, blockHash, blockNumber string) (string, error) {
	req, err := c.newRequest("GET", "/get_storage_at?contractAddress="+contractAddress+"}&key="+key+"&"+
		getFormattedBlockIdentifier(blockHash, blockNumber), nil)
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

// getTransactionStatus creates a new request to get a transaction Status
func (c Client) getTransactionStatus(txHash, txId string) (interface{}, error) {
	req, err := c.newRequest("GET", "/get_transaction_status?"+TxnIdentifier(txHash, txId), nil)
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

// getTransaction creates a new request to get a TransactionInfo
func (c Client) getTransaction(txHash, txId string) (TransactionInfo, error) {
	req, err := c.newRequest("GET", "/get_transaction?"+TxnIdentifier(txHash, txId), nil)
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

// getTransactionReceipt creates a new request to get a TransactionReceipt
func (c Client) getTransactionReceipt(txHash, txId string) (TransactionReceipt, error) {
	req, err := c.newRequest("GET", "/get_transaction_receipt?"+TxnIdentifier(txHash, txId), nil)
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

// getBlockHashById creates a new request to get block Hash based on block ID
func (c Client) getBlockHashById(blockId int64) (string, error) {
	req, err := c.newRequest("GET", "/get_block_hash_by_id?blockId="+
		strconv.FormatInt(blockId, 10), nil)
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

// getBlockIdByHash creates a new request to get Block ID based on Block Hash
func (c Client) getBlockIdByHash(blockHash string) (int64, error) {
	req, err := c.newRequest("GET", "/get_block_id_by_hash?blockHash="+blockHash, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return -1, err
	}
	var response int64
	_, err = c.do(req, &response)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return -1, err
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

// Package feeder represent a client for the Feeder Gateway connection.
// For more details of the implementation, see this client https://github.com/starkware-libs/cairo-lang/blob/master/src/starkware/starknet/services/api/feeder_gateway/feeder_gateway_client.py.
package feeder

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/NethermindEth/juno/internal/errpkg"
	"github.com/NethermindEth/juno/internal/log"
)

// XXX: Instead of string concatenation, a cleaner solution might be
// using fmt.Sprintf to compose the URLs. That way, even we even get the
// additional benefit of input validation.

const badBaseURL = "Bad base url"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . FeederHttpClient
type FeederHttpClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Client is a client for the StarkNet Feeder Gateway.
type Client struct {
	httpClient *FeederHttpClient

	BaseURL   *url.URL
	UserAgent string
	BaseAPI   string
}

// NewClient returns a new Client.
func NewClient(baseURL, baseAPI string, client *FeederHttpClient) *Client {
	u, err := url.Parse(baseURL)
	errpkg.CheckFatal(err, badBaseURL)
	if client == nil {
		var p FeederHttpClient
		c := http.Client{
			Timeout: 10 * time.Second,
		}
		p = &c
		client = &p
	}
	return &Client{BaseURL: u, BaseAPI: baseAPI, httpClient: client}
}

func fmtBlockID(blockHash, blockNumber string) string {
	if len(blockHash) == 0 {
		// XXX: See comment at top of file.
		return "blockNumber=" + blockNumber
	}
	// XXX: See comment at top of file.
	return "blockHash=" + blockHash
}

// XXX: Document or "unexport" if not used externally.
func TxnID(txHash, txID string) string {
	if len(txHash) == 0 {
		return "transactionId=" + txID
	}
	return "transactionHash=" + txHash

}

// newRequest creates a new request based on params or returns an error
// otherwise.
func (c *Client) newRequest(method, path string, body any) (*http.Request, error) {
	rel := &url.URL{Path: c.BaseAPI + path}
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

// do executes a request and waits for response or returns and error
// otherwise.
func (c *Client) do(req *http.Request, v any) (*http.Response, error) {
	res, err := (*c.httpClient).Do(req)
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
	}(res.Body)
	err = json.NewDecoder(res.Body).Decode(v)
	return res, err
}

// GetContractAddresses creates a new request to get a contract
// addresses from the gateway.
func (c Client) GetContractAddresses() (ContractAddresses, error) {
	log.Default.With("Gateway Url", c.BaseURL).Info("Getting contract address from gateway")
	req, err := c.newRequest("GET", "get_contract_addresses", nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return ContractAddresses{}, err
	}
	var res ContractAddresses
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return ContractAddresses{}, err
	}
	return res, err
}

// CallContract creates a new request to call a contract in the gateway.
func (c Client) CallContract(invokeFunction InvokeFunction, blockHash, blockNumber string) (map[string][]string, error) {
	req, err := c.newRequest(
		"POST",
		"/call_contract?"+fmtBlockID(blockHash, blockNumber),
		invokeFunction,
	)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res map[string][]string
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return res, err
}

// GetBlock creates a new request to get a block from the gateway.
func (c Client) GetBlock(blockHash, blockNumber string) (StarknetBlock, error) {
	req, err := c.newRequest(
		// XXX: See comment at top of file.
		"GET", "/get_block?"+fmtBlockID(blockHash, blockNumber), nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return StarknetBlock{}, err
	}
	var res StarknetBlock
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return StarknetBlock{}, err
	}
	return res, err
}

// GetStateUpdate creates a new request to get contract addresses from
// the gateway.
func (c Client) GetStateUpdate(blockHash, blockNumber string) (StateUpdateResponse, error) {
	req, err := c.newRequest(
		// XXX: See comment at top of file.
		"GET", "/get_state_update?"+fmtBlockID(blockHash, blockNumber), nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return StateUpdateResponse{}, err
	}
	var res StateUpdateResponse
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return StateUpdateResponse{}, err
	}
	return res, err
}

// GetCode creates a new request to get code of the contract address.
func (c Client) GetCode(contractAddress, blockHash, blockNumber string) ([]string, error) {
	req, err := c.newRequest(
		"GET",
		// XXX: See comment at top of file.
		"/get_code?contractAddress="+contractAddress+"&"+fmtBlockID(blockHash, blockNumber),
		nil,
	)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res []string
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return res, err
}

// GetFullContract creates a new request to get the full state of a
// contract.
func (c Client) GetFullContract(contractAddress, blockHash, blockNumber string) (any, error) {
	req, err := c.newRequest(
		"GET",
		// XXX: See comment at top of file.
		"/get_full_contract?contractAddress="+contractAddress+"&"+fmtBlockID(blockHash, blockNumber),
		nil,
	)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res any
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return res, err
}

// GetStorageAt creates a new request to get the storage of a contract.
func (c Client) GetStorageAt(contractAddress, key, blockHash, blockNumber string) (string, error) {
	req, err := c.newRequest(
		"GET",
		// XXX: See comment at top of file.
		"/get_storage_at?contractAddress="+contractAddress+"}&key="+key+"&"+fmtBlockID(blockHash, blockNumber),
		nil,
	)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return "", err
	}
	var res string
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return "", err
	}
	return res, err
}

// GetTransactionStatus creates a new request to get a transaction Status
func (c Client) GetTransactionStatus(txHash, txID string) (any, error) {
	req, err := c.newRequest(
		// XXX: See comment at top of file.
		"GET", "/get_transaction_status?"+TxnID(txHash, txID), nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res any
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return nil, err
	}
	return res, err
}

// GetTransaction creates a new request to get a TransactionInfo.
func (c Client) GetTransaction(txHash, txID string) (TransactionInfo, error) {
	// XXX: See comment at top of file.
	req, err := c.newRequest("GET", "/get_transaction?"+TxnID(txHash, txID), nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return TransactionInfo{}, err
	}
	var res TransactionInfo
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return TransactionInfo{}, err
	}
	return res, err
}

// GetTransactionReceipt creates a new request to get a
// TransactionReceipt.
func (c Client) GetTransactionReceipt(txHash, txID string) (TransactionReceipt, error) {
	req, err := c.newRequest(
		// XXX: See comment at top of file.
		"GET", "/get_transaction_receipt?"+TxnID(txHash, txID), nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return TransactionReceipt{}, err
	}
	var res TransactionReceipt
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return TransactionReceipt{}, err
	}
	return res, err
}

// GetBlockHashByID creates a new request to get block Hash by block ID.
func (c Client) GetBlockHashByID(blockID string) (string, error) {
	// XXX: See comment at top of file.
	req, err := c.newRequest("GET", "/get_block_hash_by_id?blockId="+blockID, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return "", err
	}
	var res string
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return "", err
	}
	return res, err
}

// GetBlockIDByHash creates a new request to get block ID by its hash.
func (c Client) GetBlockIDByHash(blockHash string) (string, error) {
	// XXX: See comment at top of file.
	req, err := c.newRequest(
		"GET", "/get_block_id_by_hash?blockHash="+blockHash, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return "", err
	}
	var res string
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return "", err
	}
	return res, err
}

// GetTransactionHashByID creates a new request to get a Transaction
// hash by its ID.
func (c Client) GetTransactionHashByID(txID string) (string, error) {
	// XXX: See comment at top of file.
	req, err := c.newRequest(
		"GET", "/get_transaction_hash_by_id?transactionId="+txID, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return "", err
	}
	var res string
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return "", err
	}
	return res, err
}

// GetTransactionIDByHash creates a new request to get a Transaction ID
// by its hash.
func (c Client) GetTransactionIDByHash(txHash string) (string, error) {
	req, err := c.newRequest(
		// XXX: See comment at top of file.
		"GET", "/get_transaction_id_by_hash?transactionHash="+txHash, nil)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Unable to create a request for get_contract_addresses.")
		return "", err
	}
	var res string
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Getaway Url", c.BaseURL).
			Error("Error connecting to getaway.")
		return "", err
	}
	return res, err
}

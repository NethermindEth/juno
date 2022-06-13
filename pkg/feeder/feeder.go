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

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . FeederHttpClient
type HttpClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Client represents a client for the StarkNet feeder gateway.
type Client struct {
	httpClient *HttpClient

	BaseURL            *url.URL
	BaseAPI, UserAgent string
}

// NewClient returns a new Client.
func NewClient(baseURL, baseAPI string, client *HttpClient) *Client {
	u, err := url.Parse(baseURL)
	errpkg.CheckFatal(err, "Bad base URL.")
	if client == nil {
		var p HttpClient
		c := http.Client{
			Timeout: 50 * time.Second,
		}
		p = &c
		client = &p
	}
	return &Client{BaseURL: u, BaseAPI: baseAPI, httpClient: client}
}

func formattedBlockIdentifier(blockHash, blockNumber string) map[string]string {
	if len(blockHash) == 0 && len(blockNumber) == 0 {
		// notest
		return nil
	}
	if len(blockHash) == 0 {
		return map[string]string{"blockNumber": blockNumber}
	}
	return map[string]string{"blockHash": blockHash}
}

func TxnIdentifier(txHash, txId string) map[string]string {
	if len(txHash) == 0 && len(txId) == 0 {
		// notest
		return nil
	}

	if len(txHash) == 0 {
		return map[string]string{"transactionId": txId}
	}
	return map[string]string{"transactionHash": txHash}
}

// newRequest creates a new request based on params and returns an
// error otherwise.
func (c *Client) newRequest(method, path string, query map[string]string, body any) (*http.Request, error) {
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

// do executes a request and waits for response and returns an error
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
			log.Default.With("Error", err).Error("Error closing body of response.")
			return
		}
	}(res.Body)
	b, err := io.ReadAll(res.Body)
	if err != nil {
		log.Default.With("Error", err).Debug("Error reading response.")
		return nil, err
	}
	err = json.Unmarshal(b, v)
	return res, err
}

// doCodeWithABI executes a request and waits for response and returns an error
// otherwise. de-Marshals response into appropriate ByteCode and ABI structs.
func (c *Client) doCodeWithABI(req *http.Request, v *CodeInfo) (*http.Response, error) {
	res, err := (*c.httpClient).Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			// notest
			log.Default.With("Error", err).Error("Error closing body of response.")
			return
		}
	}(res.Body)
	b, err := io.ReadAll(res.Body)
	if err != nil {
		log.Default.With("Error", err).Debug("Error reading response.")
		return nil, err
	}

	var reciever map[string]interface{}

	if err := json.Unmarshal(b, &reciever); err != nil {
		log.Default.With("Error", err).Debug("Error recieving unmapped input.")
		return nil, err
	}

	// unmarshal bytecode
	json.Unmarshal(b, &v)

	// separate "abi" bytes
	abi_interface := reciever["abi"]
	p, err := json.Marshal(abi_interface)

	// Unmarshal Abi bytes into Abi object
	if err := v.Abi.UnmarshalAbiJSON(p); err != nil {
		log.Default.With("Error", err).Debug("Error reading abi")
		return nil, err
	}

	return res, err
}

// GetContractAddresses creates a new request to get contract addresses
// from the gateway.
func (c Client) GetContractAddresses() (*ContractAddresses, error) {
	log.Default.With("Gateway URL", c.BaseURL).Info("Getting contract address from gateway.")
	req, err := c.newRequest("GET", "/get_contract_addresses", nil, nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res ContractAddresses
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// CallContract creates a new request to call a contract using the gateway.
func (c Client) CallContract(invokeFunc InvokeFunction, blockHash, blockNumber string) (*map[string][]string, error) {
	req, err := c.newRequest("POST", "/call_contract", formattedBlockIdentifier(blockHash, blockNumber), invokeFunc)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res map[string][]string
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetBlock creates a new request to get a block from the gateway.
func (c Client) GetBlock(blockHash, blockNumber string) (*StarknetBlock, error) {
	log.Default.Info(formattedBlockIdentifier(blockHash, blockNumber))
	req, err := c.newRequest("GET", "/get_block", formattedBlockIdentifier(blockHash, blockNumber), nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res StarknetBlock
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetStateUpdate creates a new request to get the State Update of a given block
// from the gateway.
func (c Client) GetStateUpdate(blockHash, blockNumber string) (*StateUpdateResponse, error) {
	req, err := c.newRequest("GET", "/get_state_update", formattedBlockIdentifier(blockHash, blockNumber), nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}

	var res StateUpdateResponse
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetCode creates a new request to get the code of a contract
func (c Client) GetCode(contractAddress, blockHash, blockNumber string) (*CodeInfo, error) {
	blockIdentifier := formattedBlockIdentifier(blockHash, blockNumber)
	if blockIdentifier == nil {
		// notest
		blockIdentifier = map[string]string{}
	}
	blockIdentifier["contractAddress"] = contractAddress
	req, err := c.newRequest("GET", "/get_code", blockIdentifier, nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res CodeInfo
	_, err = c.doCodeWithABI(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetFullContract creates a new request to get the full state of a
// contract.
func (c Client) GetFullContract(contractAddress, blockHash, blockNumber string) (map[string]interface{}, error) {
	blockIdentifier := formattedBlockIdentifier(blockHash, blockNumber)
	if blockIdentifier == nil {
		// notest
		blockIdentifier = map[string]string{}
	}
	blockIdentifier["contractAddress"] = contractAddress
	req, err := c.newRequest("GET", "/get_full_contract", blockIdentifier, nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res map[string]interface{}
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return res, err
}

// GetStorageAt creates a new request to get contract storage.
func (c Client) GetStorageAt(contractAddress, key, blockHash, blockNumber string) (*StorageInfo, error) {
	blockIdentifier := formattedBlockIdentifier(blockHash, blockNumber)
	if blockIdentifier == nil {
		// notest
		blockIdentifier = map[string]string{}
	}
	blockIdentifier["contractAddress"] = contractAddress
	blockIdentifier["key"] = key

	req, err := c.newRequest("GET", "/get_storage_at",
		blockIdentifier, nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res StorageInfo
	_, err = c.do(req, &res)

	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetTransactionStatus creates a new request to get the transaction
// status.
func (c Client) GetTransactionStatus(txHash, txID string) (*TransactionStatus, error) {
	req, err := c.newRequest("GET", "/get_transaction_status", TxnIdentifier(txHash, txID), nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction_status.")
		return nil, err
	}
	var res TransactionStatus
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetTransaction creates a new request to get a TransactionInfo.
func (c Client) GetTransaction(txHash, txID string) (*TransactionInfo, error) {
	req, err := c.newRequest("GET", "/get_transaction", TxnIdentifier(txHash, txID), nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction.")
		return nil, err
	}
	var res TransactionInfo
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetTransactionReceipt creates a new request to get a
// TransactionReceipt.
func (c Client) GetTransactionReceipt(txHash, txID string) (*TransactionReceipt, error) {
	req, err := c.newRequest("GET", "/get_transaction_receipt", TxnIdentifier(txHash, txID), nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction_receipt.")
		return nil, err
	}
	var res TransactionReceipt
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetTransactionTrace creates a new request to get a
// TransactionTrace (internal call info).
func (c Client) GetTransactionTrace(txHash, txID string) (*TransactionTrace, error) {
	req, err := c.newRequest("GET", "/get_transaction_trace", TxnIdentifier(txHash, txID), nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction_trace.")
		return nil, err
	}
	var res TransactionTrace
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetBlockHashById creates a new request to get block hash by on ID.
func (c Client) GetBlockHashById(blockID string) (*string, error) {
	req, err := c.newRequest(
		"GET", "/get_block_hash_by_id", map[string]string{"blockId": blockID}, nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_block_hash_by_id.")
		return nil, err
	}
	var res string
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetBlockIDByHash creates a new request to get the block ID by hash.
func (c Client) GetBlockIDByHash(blockHash string) (*string, error) {
	req, err := c.newRequest(
		"GET", "/get_block_id_by_hash", map[string]string{"blockHash": blockHash}, nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_block_id_by_hash.")
		return nil, err
	}
	var res string
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetTransactionHashByID creates a new request to get a transaction
// hash by ID.
func (c Client) GetTransactionHashByID(txID string) (*string, error) {
	req, err := c.newRequest(
		"GET", "/get_transaction_hash_by_id",
		map[string]string{"transactionId": txID}, nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction_hash_by_id.")
		return nil, err
	}
	var res string
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	return &res, err
}

// GetTransactionHashByID creates a new request to get a transaction
// hash by ID.
func (c Client) GetTransactionIDByHash(txID string) (*string, error) {
	req, err := c.newRequest(
		"GET", "/get_transaction_id_by_hash",
		map[string]string{"transactionId": txID}, nil)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction_hash_by_id.")
		return nil, err
	}
	var res json.Number
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	res_string := res.String()
	return &res_string, err
}

// func (c Client) EstimateFee(contractAddress, contractABI, functionToCall, functionInputs string) (*string, error) {
// 	// FIXME: Not working correctly at the moment.

// 	req, err := c.newRequest(
// 		"POST", "/estimate_fee",
// 		map[string]string{"contract_address": contractAddress}, nil)
// 	if err != nil {
// 		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for estimate_fee.")
// 		return nil, err
// 	}
// 	var res string
// 	_, err = c.do(req, &res)
// 	if err != nil {
// 		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
// 		return nil, err
// 	}
// 	return &res, err
// }

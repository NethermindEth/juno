// Package feeder represent a client for the Feeder Gateway connection.
// For more details of the implementation, see this client https://github.com/starkware-libs/cairo-lang/blob/master/src/starkware/starknet/services/api/feeder_gateway/feeder_gateway_client.py.
package feeder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/NethermindEth/juno/internal/errpkg"
	"github.com/NethermindEth/juno/internal/log"
	metr "github.com/NethermindEth/juno/internal/metrics/prometheus"
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
	metr.IncreaseRequestsSent()
	res, err := (*c.httpClient).Do(req)
	// notest
	for i := 0; err != nil && i < 2; i++ {
		time.Sleep(time.Second * 5)
		res, err = (*c.httpClient).Do(req)
	}
	// We tried three times and still received an error
	if err != nil {
		metr.IncreaseRequestsFailed()
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			// notest
			metr.IncreaseRequestsFailed()
			log.Default.With("Error", err).Error("Error closing body of response.")
			return
		}
	}(res.Body)
	b, err := io.ReadAll(res.Body)
	if err != nil {
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err).Debug("Error reading response.")
		return nil, err
	}
	err = json.Unmarshal(b, v)
	metr.IncreaseRequestsReceived()
	return res, err
}

// doCodeWithABI executes a request and waits for response and returns an error
// otherwise. de-Marshals response into appropriate ByteCode and ABI structs.
func (c *Client) doCodeWithABI(req *http.Request, v *CodeInfo) (*http.Response, error) {
	metr.IncreaseABISent()
	res, err := (*c.httpClient).Do(req)
	if err != nil {
		metr.IncreaseABIFailed()
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			metr.IncreaseABIFailed()
			// notest
			log.Default.With("Error", err).Error("Error closing body of response.")
			return
		}
	}(res.Body)
	b, err := io.ReadAll(res.Body)
	if err != nil {
		metr.IncreaseABIFailed()
		log.Default.With("Error", err).Debug("Error reading response.")
		return nil, err
	}

	var reciever map[string]interface{}

	if err := json.Unmarshal(b, &reciever); err != nil {
		metr.IncreaseABIFailed()
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
		metr.IncreaseABIFailed()
		log.Default.With("Error", err).Debug("Error reading abi")
		return nil, err
	}
	metr.IncreaseABIReceived()
	return res, err
}

// GetContractAddresses creates a new request to get contract addresses
// from the gateway.
func (c Client) GetContractAddresses() (*ContractAddresses, error) {
	log.Default.With("Gateway URL", c.BaseURL).Info("Getting contract address from gateway.")
	req, err := c.newRequest("GET", "/get_contract_addresses", nil, nil)
	if err != nil {
		metr.IncreaseContractAddressesFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res ContractAddresses
	metr.IncreaseContractAddressesSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseContractAddressesFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseContractAddressesReceived()
	return &res, err
}

// CallContract creates a new request to call a contract using the gateway.
func (c Client) CallContract(invokeFunc InvokeFunction, blockHash, blockNumber string) (*map[string][]string, error) {
	req, err := c.newRequest("POST", "/call_contract", formattedBlockIdentifier(blockHash, blockNumber), invokeFunc)
	if err != nil {
		metr.IncreaseContractCallsFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res map[string][]string
	metr.IncreaseContractCallsSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseContractCallsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseContractCallsReceived()
	return &res, err
}

// GetBlock creates a new request to get a block from the gateway.
func (c Client) GetBlock(blockHash, blockNumber string) (*StarknetBlock, error) {
	req, err := c.newRequest("GET", "/get_block", formattedBlockIdentifier(blockHash, blockNumber), nil)
	if err != nil {
		metr.IncreaseBlockFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res StarknetBlock
	metr.IncreaseBlockSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseBlockFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseBlockReceived()
	return &res, err
}

// GetStateUpdateGoerli creates a new request to get the contract addresses
// from the gateway.
func (c Client) GetStateUpdateGoerli(blockHash, blockNumber string) (*StateUpdateResponse, error) {
	req, err := c.newRequest("GET", "/get_state_update", formattedBlockIdentifier(blockHash, blockNumber), nil)
	if err != nil {
		metr.IncreaseStateUpdateGoerliFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res StateUpdateResponseGoerli
	metr.IncreaseStateUpdateGoerliSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseStateUpdateGoerliFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseStateUpdateGoerliReceived()
	return stateUpdateResponseToGoerli(res), err
}

func stateUpdateResponseToGoerli(res StateUpdateResponseGoerli) *StateUpdateResponse {
	deployedContracts := make([]DeployedContract, 0)

	for _, d := range res.StateDiff.DeployedContracts {
		deployedContracts = append(deployedContracts, DeployedContract{
			Address:      d.Address,
			ContractHash: d.ContractHash,
		})
	}
	return &StateUpdateResponse{
		BlockHash: res.BlockHash,
		NewRoot:   res.NewRoot,
		OldRoot:   res.OldRoot,
		StateDiff: StateDiff{
			DeployedContracts: deployedContracts,
			StorageDiffs:      res.StateDiff.StorageDiffs,
		},
	}
}

// GetStateUpdate creates a new request to get the State Update of a given block
// from the gateway.
func (c Client) GetStateUpdate(blockHash, blockNumber string) (*StateUpdateResponse, error) {
	req, err := c.newRequest("GET", "/get_state_update", formattedBlockIdentifier(blockHash, blockNumber), nil)
	if err != nil {
		metr.IncreaseStateUpdateFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}

	var res StateUpdateResponse
	metr.IncreaseStateUpdateSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseStateUpdateFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseStateUpdateReceived()
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
		metr.IncreaseABIFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_contract_addresses.")
		return nil, err
	}
	var res CodeInfo
	_, err = c.doCodeWithABI(req, &res)
	if err != nil {
		metr.IncreaseABIFailed()
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
		metr.IncreaseFullContractsFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_full_contract.")
		return nil, err
	}
	var res map[string]interface{}
	metr.IncreaseFullContractsSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseFullContractsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseFullContractsReceived()
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
		metr.IncreaseContractStorageFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_storage_at.")
		return nil, err
	}
	var res StorageInfo
	metr.IncreaseContractStorageSent()
	_, err = c.do(req, &res)

	if err != nil {
		metr.IncreaseContractStorageFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseContractStorageReceived()
	return &res, err
}

// GetTransactionStatus creates a new request to get the transaction
// status.
func (c Client) GetTransactionStatus(txHash, txID string) (*TransactionStatus, error) {
	req, err := c.newRequest("GET", "/get_transaction_status", TxnIdentifier(txHash, txID), nil)
	if err != nil {
		metr.IncreaseTxStatusFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction_status.")
		return nil, err
	}
	var res TransactionStatus
	metr.IncreaseTxStatusSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseTxStatusFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseTxStatusReceived()
	return &res, err
}

// GetTransactionTrace creates a new request to get the transaction
// trace (internal call information).
// notest
func (c Client) GetTransactionTrace(txHash, txID string) (*TransactionTrace, error) {
	req, err := c.newRequest("GET", "/get_transaction_trace", TxnIdentifier(txHash, txID), nil)
	if err != nil {
		metr.IncreaseTxTraceFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction_trace.")
		return nil, err
	}
	var res TransactionTrace
	metr.IncreaseTxTraceSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseTxTraceFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseTxTraceReceived()
	return &res, err
}

// GetTransaction creates a new request to get a TransactionInfo.
func (c Client) GetTransaction(txHash, txID string) (*TransactionInfo, error) {
	req, err := c.newRequest("GET", "/get_transaction", TxnIdentifier(txHash, txID), nil)
	if err != nil {
		metr.IncreaseTxFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction.")
		return nil, err
	}
	var res TransactionInfo
	metr.IncreaseTxSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseTxFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseTxReceived()
	return &res, err
}

// GetTransactionReceipt creates a new request to get a
// TransactionReceipt.
func (c Client) GetTransactionReceipt(txHash, txID string) (*TransactionReceipt, error) {
	req, err := c.newRequest("GET", "/get_transaction_receipt", TxnIdentifier(txHash, txID), nil)
	if err != nil {
		metr.IncreaseTxReceiptFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction_receipt.")
		return nil, err
	}
	var res TransactionReceipt
	metr.IncreaseTxReceiptSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseTxReceiptFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseTxReceiptReceived()
	return &res, err
}

// GetBlockHashById creates a new request to get block hash by on ID.
func (c Client) GetBlockHashById(blockID string) (*string, error) {
	req, err := c.newRequest(
		"GET", "/get_block_hash_by_id", map[string]string{"blockId": blockID}, nil)
	if err != nil {
		metr.IncreaseBlockHashFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_block_hash_by_id.")
		return nil, err
	}
	var res string
	metr.IncreaseBlockHashSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseBlockHashFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseBlockHashReceived()
	return &res, err
}

// GetBlockIDByHash creates a new request to get the block ID by hash.
// notest
func (c Client) GetBlockIDByHash(blockHash string) (*string, error) {
	req, err := c.newRequest(
		"GET", "/get_block_id_by_hash", map[string]string{"blockHash": blockHash}, nil)
	if err != nil {
		metr.IncreaseBlockIDFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_block_id_by_hash.")
		return nil, err
	}
	var res interface{}
	metr.IncreaseBlockIDSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseBlockIDFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	resStr := fmt.Sprintf("%v", res)
	metr.IncreaseBlockIDReceived()
	return &resStr, err
}

// GetTransactionHashByID creates a new request to get a transaction
// hash by ID.
func (c Client) GetTransactionHashByID(txID string) (*string, error) {
	req, err := c.newRequest(
		"GET", "/get_transaction_hash_by_id",
		map[string]string{"transactionId": txID}, nil)
	if err != nil {
		metr.IncreaseTxHashFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction_hash_by_id.")
		return nil, err
	}
	var res string
	metr.IncreaseTxHashSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseTxHashFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Error connecting to the gateway.")
		return nil, err
	}
	metr.IncreaseTxHashReceived()
	return &res, err
}

// GetTransactionIDByHash creates a new request to get a transaction ID
// by hash.
func (c Client) GetTransactionIDByHash(txHash string) (*string, error) {
	req, err := c.newRequest(
		"GET", "/get_transaction_id_by_hash",
		map[string]string{"transactionHash": txHash}, nil)
	if err != nil {
		metr.IncreaseTxIDFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for get_transaction_id_by_hash.")
		return nil, err
	}
	// Need to use interface as response due to response being integer or string.
	var res interface{}
	metr.IncreaseTxIDSent()
	_, err = c.do(req, &res)
	if err != nil {
		metr.IncreaseTxIDFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).
			Error("Error connecting to the gateway.")
		return nil, err
	}
	resStr := fmt.Sprintf("%v", res)
	metr.IncreaseTxIDReceived()
	return &resStr, err
}

// EstimateFee makes a POST request to retrieve expected fee from a given transaction
func (c Client) EstimateTransactionFee(contractAddress, entryPointSelector, callData, signature string) (*EstimateFeeResponse, error) {
	// Request needs header with formatted block ID. Even if empty
	blockIdentifier := formattedBlockIdentifier("", "")
	if blockIdentifier == nil {
		// notest
		blockIdentifier = map[string]string{}
	}

	callDataList := []string{callData}
	signatureList := []string{signature}

	reqBody := map[string]interface{}{
		"contract_address":     contractAddress,
		"entry_point_selector": entryPointSelector,
		"calldata":             callDataList,
		"signature":            signatureList,
	}

	req, err := c.newRequest(
		"POST", "/estimate_fee", blockIdentifier, reqBody)
	if err != nil {
		metr.EstimateFeeFailed()
		metr.IncreaseRequestsFailed()
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).Error("Unable to create a request for estimate_fee.")
		return nil, err
	}
	var res EstimateFeeResponse
	_, err = c.do(req, &res)
	if err != nil {
		log.Default.With("Error", err, "Gateway URL", c.BaseURL).
			Error("Error connecting to gateway.")
	}
	return &res, err
}

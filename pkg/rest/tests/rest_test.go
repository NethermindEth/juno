// package rest_test
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/feeder/feederfakes"
	"github.com/NethermindEth/juno/pkg/rest"
	"github.com/bxcodec/faker"
	"gotest.tools/assert"
)

var (
	httpClient         = &feederfakes.FakeHttpClient{}
	failHttpClient     = &feederfakes.FailHttpClient{}
	client             *feeder.Client
	failClient         *feeder.Client
	restHandler        = rest.RestHandler{}
	failRestHandler    = rest.RestHandler{}
	failRequestTimeout = time.Millisecond * 400
	baseUrl            = "https://localhost:8100"
	baseApi            = "/feeder_gateway/"
)

func init() {
	var p feeder.HttpClient
	p = httpClient
	var pf feeder.HttpClient
	pf = failHttpClient
	client = feeder.NewClient(baseUrl, baseApi, &p)
	restHandler.RestFeeder = client
	failClient = feeder.NewClientWithRetryFuncForDoReq(baseUrl, baseApi, &pf, func(req *http.Request, httpClient feeder.HttpClient, err error) (*http.Response, error) {
		time.Sleep(failRequestTimeout)
		return httpClient.Do(req)
	})
	failRestHandler.RestFeeder = failClient
}

func generateResponse(body string) *http.Response {
	return &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          ioutil.NopCloser(bytes.NewBufferString(body)),
		ContentLength: int64(len(body)),
		Header:        make(http.Header, 0),
	}
}

func StructFaker(a interface{}) (string, error) {
	s := reflect.ValueOf(a)
	err := faker.FakeData(&s)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// TestRestClient
func TestRestClient(t *testing.T) {
	r := rest.NewServer(":8100", "http://localhost/", "feeder_gateway")
	go func() {
		_ = r.ListenAndServe()
	}()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	r.Close(ctx)
	cancel()
}

// TestRestClientRetryFunction
func TestRestClientRetryFunction(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_block"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockNumber", "1")
	rq.Add("blockHash", "hash")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	errorMessage := "feeder gateway failed"
	httpClient.DoReturns(nil, fmt.Errorf("%s", errorMessage))

	// Send Request expecting an error
	restHandler.GetBlock(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:"+errorMessage)
}

//---------------------------------------------
//------------------TestHandlers---------------
//---------------------------------------------

// TestGetBlockHandler
func TestGetBlockHandler(t *testing.T) {
	// Build Request
	queryStr := "http://localhost:8100/feeder_gateway/get_block"
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockNumber", "1")
	rq.Add("blockHash", "hash")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Build Fake Response
	a := feeder.StarknetBlock{}
	err = faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal()
	}

	// Get Block from rest API
	restHandler.GetBlock(rr, req)

	// Check if errors were returned
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Read Rest API Response
	var cOrig feeder.StarknetBlock
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert actual equals expected
	assert.DeepEqual(t, &a, &cOrig)
}

// TestGetCodeHandler
func TestGetCodeHandler(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_code"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockNumber", "1")
	rq.Add("blockHash", "hash")
	rq.Add("contractAddress", "0x777")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Build Fake Response
	a := feeder.CodeInfo{}
	body, err := StructFaker(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(body), nil)
	if err != nil {
		t.Fatal()
	}

	// Get Code from Rest API
	restHandler.GetCode(rr, req)
	if err != nil {
		t.Fatal()
	}

	// Read Response into CodeInfo object
	var cOrig feeder.CodeInfo
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert expected matches actual
	assert.DeepEqual(t, &a, &cOrig)
}

// TestGetStorageAtHandler
func TestGetStorageAtHandler(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_storage_at"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockNumber", "0")
	rq.Add("blockHash", "hash")
	rq.Add("contractAddress", "address")
	rq.Add("key", "key")
	req.URL.RawQuery = rq.Encode()

	// Build Response object
	rr := httptest.NewRecorder()

	// Create Fake Response
	var b feeder.StorageInfo
	err = faker.FakeData(&b)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(b)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	var a feeder.StorageInfo
	err = json.Unmarshal([]byte(body), &a)
	if err != nil {
		t.Fatal()
	}

	// Get Storage from Rest API
	restHandler.GetStorageAt(rr, req)
	println(rr.Body.String())
	if err != nil {
		t.Fatal()
	}

	// Read Response into StorageInfo struct
	var cOrig feeder.StorageInfo
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert Actual equals Expected
	assert.DeepEqual(t, &a, &cOrig)
}

// TestGetTransactionStatusHandler
func TestGetTransactionStatusHandler(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_status"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("transactionHash", "hash")
	rq.Add("txId", "id")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Build Fake Response
	a := feeder.TransactionStatus{}
	err = faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	var b feeder.TransactionStatus
	err = json.Unmarshal([]byte(body), &b)
	if err != nil {
		t.Fatal()
	}

	// Get Transaction Status from Rest API
	restHandler.GetTransactionStatus(rr, req)
	if err != nil {
		t.Fatal()
	}

	// Read Rest API Response
	var cOrig feeder.TransactionStatus
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert Actual equals Expected
	assert.DeepEqual(t, &b, &cOrig)
}

// TestGetTransactionReceiptHandler
func TestGetTransactionRecieptHandler(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_receipt"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("transactionHash", "hash")
	rq.Add("txId", "id")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Build Fake Response
	a := feeder.TransactionReceipt{}
	err = faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	var b feeder.TransactionReceipt
	err = json.Unmarshal([]byte(body), &b)
	if err != nil {
		t.Fatal()
	}

	// Get Transaction Reciept from Rest API
	restHandler.GetTransactionReceipt(rr, req)
	if err != nil {
		t.Fatal()
	}

	// Read Rest API Response
	var cOrig feeder.TransactionReceipt
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert Actual equals Expected
	assert.DeepEqual(t, &b, &cOrig)
}

// TestGetTransactionHandler
func TestGetTransactionHandler(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction"
	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("transactionHash", "hash")
	rq.Add("txId", "id")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Build Fake Response
	a := feeder.TransactionInfo{}
	err = faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	var b feeder.TransactionInfo
	err = json.Unmarshal([]byte(body), &b)
	if err != nil {
		t.Fatal()
	}

	// Get Transaction from Rest API
	restHandler.GetTransaction(rr, req)
	if err != nil {
		t.Fatal()
	}

	// Read Rest API Response
	var cOrig feeder.TransactionInfo
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert Actual equals Expected
	assert.DeepEqual(t, &b, &cOrig)
}

// TestGetFullContractHandler
func TestGetFullContractHandler(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_full_contract"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockNumber", "0")
	rq.Add("blockHash", "hash")
	rq.Add("contractAddress", "address")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	body := "{\"program\": {\"prime\": \"3\", \"data\": [\"1\", \"2\"]}, \"entry_points_by_type\": {\"constructor\": \"3\", \"external\": [\"1\", \"2\"]}, \"abi\": [\"1\", \"2\"]}\n"
	httpClient.DoReturns(generateResponse(body), nil)
	var a map[string]interface{}
	err = json.Unmarshal([]byte(body), &a)
	if err != nil {
		t.Fatal()
	}

	// Get Full Contract from Rest API
	restHandler.GetFullContract(rr, req)
	if err != nil {
		t.Fatal()
	}
	// Read Rest API Response
	var cOrig map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert Actual equals Expected
	assert.DeepEqual(t, &a, &cOrig)
}

// TestGetStateUpdateHandler
func TestGetStateUpdateHandler(t *testing.T) {
	// Build Request
	queryStr := "http://localhost:8100/feeder_gateway/get_state_update"
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockNumber", "1")
	rq.Add("blockHash", "hash")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Build Fake Response
	a := feeder.StateUpdateResponse{}
	err = faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal()
	}

	// Get State Update from rest API
	restHandler.GetStateUpdate(rr, req)

	// Check if errors were returned
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Read Rest API Response
	var cOrig feeder.StateUpdateResponse
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert actual equals expected
	assert.DeepEqual(t, &a, &cOrig)
}

// TestGetContractAddressesHandler
func TestGetContractAddressesHandler(t *testing.T) {
	// Build Request
	queryStr := "http://localhost:8100/feeder_gateway/get_contract_addresses"
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Build Response Object
	rr := httptest.NewRecorder()

	// Build Fake Response
	a := feeder.ContractAddresses{}
	err = faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal()
	}

	// Get Contract Address from rest API
	restHandler.GetContractAddresses(rr, req)

	// Check if errors were returned
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Read Rest API Response
	var cOrig feeder.ContractAddresses
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert actual equals expected
	assert.DeepEqual(t, &a, &cOrig)
}

// TestGetBlockHashByIDHandler
func TestGetBlockHashbyIdHandler(t *testing.T) {
	// Build Request
	queryStr := "http://localhost:8100/feeder_gateway/get_block_hash_by_id"
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockId", "1")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Build Fake Response
	var a string
	err = faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal()
	}

	// Get Contract Address from rest API
	restHandler.GetBlockHashById(rr, req)

	// Check if errors were returned
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Read Rest API Response
	var cOrig string
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert actual equals expected
	assert.DeepEqual(t, &a, &cOrig)
}

// TestGetBlockIDbyHashHandler
func TestGetBlockIDByHashHandler(t *testing.T) {
	// Build Request
	queryStr := "http://localhost:8100/feeder_gateway/get_block_id_by_hash"
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockHash", "10001")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Build Fake Response
	var a string
	err = faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal()
	}

	// Get Contract Address from rest API
	restHandler.GetBlockIDByHash(rr, req)

	// Check if errors were returned
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Read Rest API Response
	var cOrig string
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert actual equals expected
	assert.DeepEqual(t, &a, &cOrig)
}

// TestGetTransactionHashbyIdHandler
func TestGetTransactionHashbyIdHandler(t *testing.T) {
	// Build Request
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_hash_by_id"
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("transactionId", "1")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Build Fake Response
	var a string
	err = faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal()
	}

	// Get Contract Address from rest API
	restHandler.GetTransactionHashByID(rr, req)

	// Check if errors were returned
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Read Rest API Response
	var cOrig string
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert actual equals expected
	assert.DeepEqual(t, &a, &cOrig)
}

// TestGetTransactionIDbyHashHandler
func TestGetTransactionIDByHashHandler(t *testing.T) {
	// Build Request
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_id_by_hash"
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("transactionHash", "10001")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Build Fake Response
	var a string
	err = faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal()
	}

	// Get Contract Address from rest API
	restHandler.GetTransactionIDByHash(rr, req)

	// Check if errors were returned
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Read Rest API Response
	var cOrig string
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	// Assert actual equals expected
	assert.DeepEqual(t, &a, &cOrig)
}

//---------------------------------------------
//------------------TestInputs-----------------
//---------------------------------------------

// TestGetBlockWithoutBlockIdentifier
func TestGetBlockWithoutBlockIdentifier(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_block"

	// Build Response without Query Args
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get Block from rest API
	restHandler.GetBlock(rr, req)

	// Assert Error is written to response object
	assert.Equal(t, rr.Body.String(), "GetBlock Request Failed: expected blockNumber or blockHash")
}

func TestGetCodeWithoutContractAddressAndBlockIdentifier(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_code"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args without contractAddress
	rq := req.URL.Query()
	rq.Add("blockNumber", "1")
	rq.Add("blockHash", "hash")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get Block from rest API
	restHandler.GetCode(rr, req)

	// Assert Error query args were not correct
	assert.Equal(t, rr.Body.String(), "GetCode Request Failed: expected (blockNumber or blockHash) and contractAddress")
}

func TestGetStorageAtWithoutKey(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args without key
	rq := req.URL.Query()
	rq.Add("blockNumber", "1")
	rq.Add("blockHash", "hash")
	rq.Add("contractAddress", "Address")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get Storage from rest API
	restHandler.GetStorageAt(rr, req)

	// Assert Error query args were not correct
	assert.Equal(t, rr.Body.String(), "GetStorageAt Request Failed: expected blockIdentifier, contractAddress, and key")
}

func TestGetTransactionStatusWithoutTransactionIdentifier(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_status"

	// Build Request without Query Args
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get Transaction Status from rest API
	restHandler.GetTransactionStatus(rr, req)

	// Assert Error query args were not correct
	assert.Equal(t, rr.Body.String(), "GetTransactionStatus failed: expected txId or transactionHash")
}

func TestGetTransactionReceiptWithoutTransactionIdentifier(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_receipt"

	// Build Request without Query Args
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get Transaction Receipt from rest API
	restHandler.GetTransactionReceipt(rr, req)

	// Assert Error query args were not correct
	assert.Equal(t, rr.Body.String(), "GetTransactionReceipt failed: expected txId or transactionHash")
}

func TestGetTransactionWithoutTransactionIdentifier(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction"

	// Build Request without Query Args
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get Transaction from rest API
	restHandler.GetTransaction(rr, req)

	// Assert Error query args were not correct
	assert.Equal(t, rr.Body.String(), "GetTransaction failed: expected txId or transactionHash")
}

func TestGetFullContractAtWithoutContractAddress(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args without contractAddress
	rq := req.URL.Query()
	rq.Add("blockNumber", "1")
	rq.Add("blockHash", "hash")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get Storage from rest API
	restHandler.GetFullContract(rr, req)

	// Assert Error query args were not correct
	assert.Equal(t, rr.Body.String(), "GetFullContract failed: expected contractAddress and Block Identifier")
}

func TestGetStateUpdateWithoutBlockIdentifier(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_state_update"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get State Update from rest API
	restHandler.GetStateUpdate(rr, req)

	// Assert Error query args were not correct
	assert.Equal(t, rr.Body.String(), "GetStateUpdate failed: expected Block Identifier")
}

// TestGetBlockHashByIDWithoutTransactionId
func TestGetBlockHashByIDWithoutTransactionId(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_block_id_by_hash"

	// Build Request without Query Args
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get Transaction Receipt from rest API
	restHandler.GetBlockHashById(rr, req)

	// Assert Error query args were not correct
	assert.Equal(t, rr.Body.String(), "GetBlockHashById failed: expected blockId")
}

// TestGetBlockIDByHashWithoutTransactionHash
func TestGetBlockIDByHashWithoutTransactionHash(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_block_id_by_hash"

	// Build Request without Query Args
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get Transaction Receipt from rest API
	restHandler.GetBlockIDByHash(rr, req)

	// Assert Error query args were not correct
	assert.Equal(t, rr.Body.String(), "GetBlockIDByHash failed: expected blockHash")
}

// TestGetTransactionHashByIDWithoutTransactionId
func TestGetTransactionHashByIDWithoutTransactionId(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_id_by_hash"

	// Build Request without Query Args
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get Transaction Receipt from rest API
	restHandler.GetTransactionHashByID(rr, req)

	// Assert Error query args were not correct
	assert.Equal(t, rr.Body.String(), "GetTransactionHashByID failed: expected transactionId")
}

// TestGetTransactionIDByHashWithoutTransactionHash
func TestGetTransactionIDByHashWithoutTransactionHash(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_id_by_hash"

	// Build Request without Query Args
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Build Response Object
	rr := httptest.NewRecorder()

	// Get Transaction Receipt from rest API
	restHandler.GetTransactionIDByHash(rr, req)

	// Assert Error query args were not correct
	assert.Equal(t, rr.Body.String(), "GetTransactionIDByHash failed: expected transactionHash")
}

//---------------------------------------------
//------------------FeederFails----------------
//---------------------------------------------

// TestGetBlockHandlerFeederFail
func TestGetBlockHandlerFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_block"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockNumber", "1")
	rq.Add("blockHash", "hash")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetBlock(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetCodeHandlerFeederFail
func TestGetCodeHandlerFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_code"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockNumber", "1")
	rq.Add("blockHash", "hash")
	rq.Add("contractAddress", "address")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetCode(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetStorageAtHandlerFeederFail
func TestGetStorageAtHandlerFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_storage_at"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockNumber", "")
	rq.Add("blockHash", "hash")
	rq.Add("contractAddress", "address")
	rq.Add("key", "key")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetStorageAt(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetTransactionStatusHandlerFeederFail
func TestGetTransactionStatusHandlerFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_status"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("transactionHash", "hash")
	rq.Add("txId", "id")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetTransactionStatus(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetTransactionReceiptHandlerFeederFail
func TestGetTransactionReceiptHandlerFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_receipt"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("transactionHash", "hash")
	rq.Add("txId", "id")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetTransactionReceipt(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetTransactionHandlerFeederFail
func TestGetTransactionHandlerFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("transactionHash", "hash")
	rq.Add("txId", "id")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetTransaction(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetFullContractHandlerFeederFail
func TestGetFullContractHandlerFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_full_contract"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockNumber", "")
	rq.Add("blockHash", "hash")
	rq.Add("contractAddress", "address")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetFullContract(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetStateUpdateHandlerFeederFail
func TestGetStateUpdateFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_state_update"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockNumber", "")
	rq.Add("blockHash", "hash")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetStateUpdate(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetContractAddressesFeederFail
func TestGetContractAddressesFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_contract_addresses"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetContractAddresses(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetBlockIDByHashFeederFail
func TestGetBlockIDByHashFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_block_id_by_hash"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockHash", "3213")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetBlockIDByHash(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetBlockHashByIDFeederFail
func TestGetBlockHashByIDFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_block_hash_by_id"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("blockId", "3213")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetBlockHashById(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetTransactionIDByHashFeederFail
func TestGetTransactionIDByHashFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_id_by_hash"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("transactionHash", "3213")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetTransactionIDByHash(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

// TestGetBlockHashByIDFeederFail
func TestGetTransactionHashByIDFeederFail(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_hash_by_id"

	// Build Request
	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Query Args
	rq := req.URL.Query()
	rq.Add("transactionId", "3213")
	req.URL.RawQuery = rq.Encode()

	// Build Response Object
	rr := httptest.NewRecorder()

	// Send Request expecting an error
	failRestHandler.GetTransactionHashByID(rr, req)

	// Assert error message
	assert.DeepEqual(t, rr.Body.String(), "Invalid request body error:feeder gateway failed")
}

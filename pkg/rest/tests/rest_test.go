// package rest_test
package tests

// NOTE: feederfakes creates an import cycle so testing has to be in a
// different package.

import (
	"bytes"
	"context"
	"encoding/json"
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
	httpClient  = &feederfakes.FakeHttpClient{}
	client      *feeder.Client
	restHandler = rest.RestHandler{}
)

func init() {
	var p feeder.HttpClient
	p = httpClient
	client = feeder.NewClient("https://localhost:8100", "/feeder_gateway/", &p)
	restHandler.RestFeeder = client
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

// You can use testing.T, if you want to test the code without benchmarking
// func setupRestTests(t testing.T) func(t testing.T) {
// 	println("setup")
// 	r := rest.NewServer(":8100", "http://localhost/")
// 	go func() {
// 		_ = r.ListenAndServe()
// 	}()
// 	// Return a function to teardown the test
// 	return func(t testing.T) {
// 		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
// 		r.Close(ctx)
// 		cancel()
// 	}
// }

//TestRestClient
func TestRestClient(t *testing.T) {
	r := rest.NewServer(":8100", "http://localhost/")
	go func() {
		_ = r.ListenAndServe()
	}()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	r.Close(ctx)
	cancel()
}

// Produces empty
// TestGetBlockHandler
func TestGetBlockHandler(t *testing.T) {

	queryStr := "http://localhost:8100/feeder_gateway/get_block"

	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	rq := req.URL.Query()
	rq.Add("blockNumber", "1")
	rq.Add("blockHash", "hash")
	req.URL.RawQuery = rq.Encode()

	rr := httptest.NewRecorder()

	//build response
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
	//httpClient.DoReturns(generateResponse(body), nil)

	//Get Block from rest API
	restHandler.GetBlock(rr, req)

	// Check if errors were returned
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	// Expect block with number 0
	var testBlock feeder.StarknetBlock
	testBlock.BlockNumber = 10

	var cOrig feeder.StarknetBlock
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	//assert.Equal(t, &cOrig.BlockNumber, &testBlock.BlockNumber, "Get Block is empty")
	assert.Equal(t, &a.BlockHash, &cOrig.BlockHash, "Get Block does not match expected")
}

// Produces empty
// TestGetCodeHandler
func TestGetCodeHandler(t *testing.T) {

	queryStr := "http://localhost:8100/feeder_gateway/get_code"

	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	rq := req.URL.Query()
	rq.Add("blockNumber", "1")
	rq.Add("blockHash", "hash")
	rq.Add("contractAddress", "0x777")
	req.URL.RawQuery = rq.Encode()

	rr := httptest.NewRecorder()

	//build response
	a := feeder.CodeInfo{}
	// body, err := StructFaker(a)
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
	//httpClient.DoReturns(generateResponse(body), nil)

	restHandler.GetCode(rr, req)
	if err != nil {
		t.Fatal()
	}
	var cOrig feeder.CodeInfo
	json.Unmarshal(rr.Body.Bytes(), &cOrig)
	assert.Equal(t, &a, &cOrig, "GetCode response don't match")
}

// produces string data but they do not match
// TestGetStorageAtHandler
func TestGetStorageAtHandler(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_storage_at"

	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	rq := req.URL.Query()
	rq.Add("blockNumber", "")
	rq.Add("blockHash", "hash")
	rq.Add("contractAddress", "address")
	rq.Add("key", "key")
	req.URL.RawQuery = rq.Encode()

	rr := httptest.NewRecorder()

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
	restHandler.GetStorageAt(rr, req)
	if err != nil {
		t.Fatal()
	}
	var cOrig feeder.StorageInfo
	json.Unmarshal(rr.Body.Bytes(), &cOrig)
	assert.Equal(t, &a, &cOrig, "Storage response don't match")
}

// Produces data, they match, but fails
// TestGetTransactionStatusHandler
func TestGetTransactionStatusHandler(t *testing.T) {
	queryStr := "http://localhost:8100/feeder_gateway/get_transaction_status"

	req, err := http.NewRequest("GET", queryStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	rq := req.URL.Query()
	rq.Add("transactionHash", "hash")
	rq.Add("txId", "id")
	req.URL.RawQuery = rq.Encode()

	rr := httptest.NewRecorder()

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
	restHandler.GetTransactionStatus(rr, req)
	if err != nil {
		t.Fatal()
	}
	var cOrig feeder.TransactionStatus
	json.Unmarshal(rr.Body.Bytes(), &cOrig)

	assert.Equal(t, &b, &cOrig, "GetTransactionStatus response don't match")
}

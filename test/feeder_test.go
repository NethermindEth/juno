package test

import (
	"bytes"
	"encoding/json"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/gateway"
	"github.com/NethermindEth/juno/pkg/gateway/gatewayfakes"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
)

var httpClient = &gatewayfakes.FakeFeederHttpClient{}
var client *gateway.Client

func init() {
	var p gateway.FeederHttpClient
	p = httpClient
	client = gateway.NewClient("https:/local", "/feeder_gateway/", &p)
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

func TestGetContractAddress(t *testing.T) {
	body := "{\"GpsStatementVerifier\":\"0x47312450B3Ac8b5b8e247a6bB6d523e7605bDb60\",\"Starknet\":\"0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4\"}\n"
	httpClient.DoReturns(generateResponse(body), nil)
	var cOrig gateway.ContractAddresses
	err := json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fail()
		return
	}
	contractAddresses, err := client.GetContractAddresses()
	if err != nil {
		return
	}
	assert.Equal(t, cOrig, contractAddresses, "Contract Address don't match")
	log.Default.With("Contract Addresses", contractAddresses).Info("Successfully getContractAddress request")
}

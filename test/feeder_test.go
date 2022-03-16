package test

import (
	"bytes"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/gateway"
	"github.com/NethermindEth/juno/pkg/gateway/gatewayfakes"
	"io/ioutil"
	"net/http"
	"testing"
)

var httpClient = &gatewayfakes.FakeFeederHttpClient{}
var client *gateway.Client

func init() {
	var p gateway.FeederHttpClient
	p = httpClient
	body := "{\"GpsStatementVerifier\":\"0x47312450B3Ac8b5b8e247a6bB6d523e7605bDb60\",\"Starknet\":\"0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4\"}\n"
	httpClient.DoReturns(&http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          ioutil.NopCloser(bytes.NewBufferString(body)),
		ContentLength: int64(len(body)),
		Header:        make(http.Header, 0),
	}, nil)
	client = gateway.NewClient("https:/local", "/feeder_gateway/", &p)
}

func TestGetContractAddress(t *testing.T) {
	contractAddresses, err := client.GetContractAddresses()
	if err != nil {
		return
	}
	log.Default.With("Contract Addresses", contractAddresses).Info("Successfully getContractAddress request")
}

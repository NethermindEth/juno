package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder_gateway"
	"github.com/NethermindEth/juno/pkg/feeder_gateway/gatewayfakes"
	"github.com/bxcodec/faker"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

var httpClient = &gatewayfakes.FakeFeederHttpClient{}
var client *feeder_gateway.Client

func init() {
	var p feeder_gateway.FeederHttpClient
	p = httpClient
	client = feeder_gateway.NewClient("https:/local", "/feeder_gateway/", &p)
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

func TestGetContractAddress(t *testing.T) {
	body := "{\"GpsStatementVerifier\":\"0x47312450B3Ac8b5b8e247a6bB6d523e7605bDb60\",\"Starknet\":\"0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4\"}\n"
	httpClient.DoReturns(generateResponse(body), nil)
	var cOrig feeder_gateway.ContractAddresses
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

func TestGetBlock(t *testing.T) {
	a := feeder_gateway.StarknetBlock{}
	body, err := StructFaker(a)
	if err != nil {
		fmt.Println(err)
		t.Fail()
		return
	}
	httpClient.DoReturns(generateResponse(body), nil)
	starknetBlock, err := client.GetBlock("", "latest")
	if err != nil {
		t.Fail()
		return
	}
	assert.Equal(t, a, starknetBlock, "StarknetBlock don't match")
	log.Default.With("Block Response", starknetBlock).Info("Successfully getBlock request")
}

// package feeder_test
package tests

// NOTE: feederfakes creates an import cycle so testing has to be in a
// different package.

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/feeder/feederfakes"
	"github.com/stretchr/testify/assert"
)

var realClient *feeder.Client

func init() {
	realClient = feeder.NewClient("https://alpha-mainnet.starknet.io", "/feeder_gateway", nil)
}

func TestRealGetBlock(t *testing.T) {
	a := feederfakes.ReturnFakeBlockInfo()
	getBlock, err := realClient.GetBlock("", "0")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, a, getBlock, "GetBlock response don't match")
}

func TestRealGetCode(t *testing.T) {
	a := feederfakes.ReturnFakeCodeInfo()
	getCode, err := realClient.GetCode("0x0090bff87efa37c2a8d0fd8b903ca1220dbb375ad18346e0da59af7f7e6c4285", "", "latest")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, a, getCode, "GetCode response don't match")
}

func TestRealGetCode_FullCoverage(t *testing.T) {
	a := feederfakes.ReturnAbiInfo_Full()
	assert.Equal(t, "Struct-custom", a.Structs[0].Name)
}

func TestRealGetCode_FailType(t *testing.T) {
	a := feederfakes.ReturnAbiInfo_Fail()
	err := fmt.Errorf("unexpected type %s", "unknown")
	assert.Equal(t, err, a)
}

func TestRealGetContractAddress(t *testing.T) {
	a := feederfakes.ReturnFakeContractAddressInfo()
	getContractAdresses, err := realClient.GetContractAddresses()
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, a, getContractAdresses, "GetCode response don't match")
}

func TestRealGetStateUpdate(t *testing.T) {
	a := feederfakes.ReturnFakeStateUpdateInfo()
	getStateUpdate, err := realClient.GetStateUpdate("", "2")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, a, getStateUpdate, "GetCode response don't match")
}

func TestRealGetTransactionReceipt(t *testing.T) {
	a := feederfakes.ReturnFakeTransactionReceiptInfo()
	getTransactionReceipt, err := realClient.GetTransactionReceipt("0xe0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75", "")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, a, getTransactionReceipt, "GetCode response don't match")
}

func TestRealGetTransaction(t *testing.T) {
	a := feederfakes.ReturnFakeTransactionInfo()
	getTransaction, err := realClient.GetTransaction("0xe0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75", "")
	if err != nil {
		t.Fatal()
	}
	println(a.TransactionInBlockInformation.BlockHash)
	assert.Equal(t, a, getTransaction, "GetCode response don't match")
}

func TestRealGetTransactionStatus(t *testing.T) {
	a := feederfakes.ReturnFakeTransactionStatusInfo()
	getTransactionStatus, err := realClient.GetTransactionStatus("0xe0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75", "")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, a, getTransactionStatus, "GetCode response don't match")
}

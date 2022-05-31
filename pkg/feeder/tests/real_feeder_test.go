// package feeder_test
package tests

// NOTE: feederfakes creates an import cycle so testing has to be in a
// different package.

import (
	"testing"

	"fmt"

	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/feeder/feederfakes"
	"github.com/stretchr/testify/assert"
)

var realClient *feeder.Client

func init() {
	realClient = feeder.NewClient("https://alpha-mainnet.starknet.io", "/feeder_gateway", nil)
}

func TestRealGetFullContract(t *testing.T) {
	//a := feederfakes.ReturnFakeFullContract()
	getBlock, err := realClient.GetFullContract("0x03a0ae1aaefeed60bafd6990f06d0b68fb593b5d9395ff726868ee61a6e1beb3", "", "3")
	if err != nil {
		t.Fatal()
	}
	//k := (getBlock
	m := getBlock["abi"]
	if m != nil {
		assert.True(t, true, "Full Contract response don't match")
	} else {
		assert.True(t, false, "Ful Contract does not contain abi - failed")
	}
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
	assert.Equal(t, a, getContractAdresses, "GetContractAddress response don't match")
}

func TestRealGetStateUpdate(t *testing.T) {
	a := feederfakes.ReturnFakeStateUpdateInfo()
	getStateUpdate, err := realClient.GetStateUpdate("", "2")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, a, getStateUpdate, "GetStateUpdate response don't match")
}

func TestRealGetTransactionReceipt(t *testing.T) {
	a := feederfakes.ReturnFakeTransactionReceiptInfo()
	getTransactionReceipt, err := realClient.GetTransactionReceipt("0xe0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75", "")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, a, getTransactionReceipt, "GetTransactionReceipt response don't match")
}

func TestRealGetTransaction(t *testing.T) {
	a := feederfakes.ReturnFakeTransactionInfo()
	getTransaction, err := realClient.GetTransaction("0xe0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75", "")
	if err != nil {
		t.Fatal()
	}
	println(a.TransactionInBlockInformation.BlockHash)
	assert.Equal(t, a, getTransaction, "GetTransaction response don't match")
}

func TestRealGetTransactionStatus(t *testing.T) {
	a := feederfakes.ReturnFakeTransactionStatusInfo()
	getTransactionStatus, err := realClient.GetTransactionStatus("0xe0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75", "")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, a, getTransactionStatus, "GetTransactionStatus response don't match")
}

func TestRealGetStorageAt(t *testing.T) {
	a := feederfakes.ReturnFakeStorageAt()
	getStorageAt, err := realClient.GetStorageAt("0x2f64e2c2650a3663169758c92d58acae85177fe218469e7e07a358f3ea1654d", "5", "0x4223f3e4f2d1e6c9753b04974acdf045e602ccfe784ea6d3722697bda0fc4d2", "100")
	if err != nil {
		t.Error(err)
		t.Fatal()
	}
	assert.Equal(t, a, getStorageAt, "GetStorageAt responses didn't match")
}

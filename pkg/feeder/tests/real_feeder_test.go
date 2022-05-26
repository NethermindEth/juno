// package feeder_test
package tests

// NOTE: feederfakes creates an import cycle so testing has to be in a
// different package.

import (
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

package test

import (
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/gateway"
	"testing"
)

func setupClient() *gateway.Client {
	return gateway.NewClient("https://alpha-mainnet.starknet.io", "/feeder_gateway/")
}

func TestGatewayClient(t *testing.T) {
	client := setupClient()
	contractAddresses, err := client.GetContractAddresses()
	if err != nil {
		return
	}
	log.Default.With("Contract Addresses", contractAddresses).Info("Successfully getContractAddress request")
}

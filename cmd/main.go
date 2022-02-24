package main

import (
	"context"
	"github.com/NethermindEth/juno/internal/log"

	"github.com/NethermindEth/juno/configs"
	"github.com/tarrencev/go-starknet/provider"
)

var logger = log.GetLogger()

func main() {
	logger.Info("Starting Juno, StarkNet Go Client")
	baseURL := configs.MainnetGateway
	prv := provider.NewProvider(baseURL)
	// opt := provider.BlockOptions{}
	ctx := context.Background()
	block, err := prv.Block(ctx, nil)
	if err != nil {
		logger.Errorw("Failed to retrieve block",
			"With Error", err)
	}
	logger.Debug("Block Hash retrieved from provider, ", "blockHash:", block.BlockHash)
}

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
		logger.With("With Error", err).Error("Failed to retrieve block")
	}
	logger.With("blockHash", block.BlockHash).Debug("Block Hash retrieved from provider, ")
}

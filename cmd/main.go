package main

import (
	"context"
	log "github.com/sirupsen/logrus"

	"github.com/NethermindEth/juno/configs"
	"github.com/tarrencev/go-starknet/provider"
)

func main() {
	log.Info("Starting Juno, StarkNet Go Client")
	baseURL := configs.MainnetGateway
	prv := provider.NewProvider(baseURL)
	// opt := provider.BlockOptions{}
	ctx := context.Background()
	block, err := prv.Block(ctx, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"With Error": err,
		}).Error("Failed to retrieve block")
	}
	log.WithFields(log.Fields{
		"blockHash": block.BlockHash,
	}).Debug("Block Hash retrieved from provider")
}

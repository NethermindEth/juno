package main

import (
	cmd "github.com/NethermindEth/juno/cmd/starknet"
)

func main() {
	cmd.Execute()
	//end := make(chan error)
	//go rpc.Handlers(end)
	//err := <-end
	//if err != nil {
	//	logger.With("Error:", err).Error("Error in the RPC Server")
	//	return
	//}
	//logger.Info("Starting Juno, StarkNet Go Client")
	//baseURL := configs.MainnetGateway
	//prv := provider.NewProvider(baseURL)
	//// opt := provider.BlockOptions{}
	//ctx := context.Background()
	//block, err := prv.Block(ctx, nil)
	//if err != nil {
	//	logger.With("With Error", err).Error("Failed to retrieve block")
	//}
	//logger.With("blockHash", block.BlockHash).Debug("Block Hash retrieved from provider, ")

}

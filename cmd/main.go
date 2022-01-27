package main

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/configs"
	"github.com/tarrencev/go-starknet/provider"
)

func main() {
	baseURL := configs.MainnetGateway
	prv := provider.NewProvider(baseURL)
	// opt := provider.BlockOptions{}
	ctx := context.Background()
	block, err := prv.Block(ctx, nil)
	if err != nil {
		fmt.Printf("Failed to retrieve block: %s", err)
	}

	fmt.Println(block.BlockHash)
}

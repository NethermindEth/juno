package main

import (
	"context"
	"fmt"
	"github.com/tarrencev/go-starknet/provider"
)

func main() {
	baseURL := mainnetGateway
	prv := provider.NewProvider(baseURL)
	// opt := provider.BlockOptions{}
	ctx := context.Background()
	block, err := prv.Block(ctx, nil)
	if err != nil {
		fmt.Printf("Failed to retrieve block: %s", err)
	}

	fmt.Println(block.BlockHash)
}

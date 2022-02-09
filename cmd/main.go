package main

import cmd "github.com/NethermindEth/juno/cmd/starknet"

func main() {
	cmd.Execute()
	//baseURL := configs.MainnetGateway
	//prv := provider.NewProvider(baseURL)
	//// opt := provider.BlockOptions{}
	//ctx := context.Background()
	//block, err := prv.Block(ctx, nil)
	//if err != nil {
	//	fmt.Printf("Failed to retrieve block: %s", err)
	//}
	//
	//fmt.Println(block.BlockHash)
}

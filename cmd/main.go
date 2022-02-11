package main

import (
	"fmt"
	cmd "github.com/NethermindEth/juno/cmd/starknet"
	"github.com/NethermindEth/juno/pkg/rpc"
)

func main() {
	fmt.Print(`      _                   
     | |                  
     | |_   _ _ __   ___  
 _   | | | | | '_ \ / _ \ 
| |__| | |_| | | | | (_) |
 \____/ \__,_|_| |_|\___/ 
                          
                          
`)
	cmd.Execute()
	end := make(chan error)
	go rpc.Handlers(end)
	err := <-end
	if err != nil {
		fmt.Printf("Error in the RPC Server: %s\n", err.Error())
		return
	}
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

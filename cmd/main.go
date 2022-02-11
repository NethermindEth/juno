package main

import (
	"fmt"
	"github.com/NethermindEth/juno/pkg/rpc"
)

func main() {
	fmt.Println("Juno, Starknet Client in Go")
	end := make(chan error)
	go rpc.Handlers(end)
	err := <-end
	if err != nil {
		fmt.Printf("Error in the RPC Server: %s\n", err.Error())
		return
	}
}

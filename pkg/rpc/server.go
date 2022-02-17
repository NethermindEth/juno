package rpc

import (
	"context"
	"fmt"
	cmd "github.com/NethermindEth/juno/cmd/starknet"
	"log"
	"net/http"
)

type Server struct{}

func Handlers(end chan error) {
	mr := NewMethodRepositoryWithMethods(Server{})

	http.Handle("/rpc", mr)

	fmt.Println("Listening for connections .... ")
	if err := http.ListenAndServe(":8080", http.DefaultServeMux); err != nil {
		log.Fatalln(err)
	}
	end <- nil
}

// Echo represents the handler of "echo" rpc call, just reply with the same message
func (Server) Echo(c context.Context, request Echo) (Echo, error) {
	return Echo{
		Message: request.Message,
	}, nil
}

// StarknetCall represents the handler of "starknet_call" rpc call
func (Server) StarknetCall(c context.Context, request cmd.RequestRPC) (cmd.ResultCall, error) {

	return []string{"Response", "of", "starknet_call"}, nil
}

// StarknetGetBlockByHash represent the handler for getting a block by its hash
func (Server) StarknetGetBlockByHash(c context.Context, p BlockHashParams) (BlockHashResult, error) {
	return BlockHashResult{}, nil
}

// StarknetGetBlockByNumber represent the handler for getting a block by its number
func (Server) StarknetGetBlockByNumber(c context.Context, p BlockNumberParams) (BlockNumberResult, error) {
	return BlockNumberResult{}, nil
}

// StarknetGetBlockTransactionCountByHash represent the handler for getting block transaction count by the blocks hash
func (Server) StarknetGetBlockTransactionCountByHash(c context.Context, p BlockTransactionCountParams) (BlockTransactionCountResult, error) {
	return BlockTransactionCountResult{}, nil
}

// StarknetGetStateUpdateByHash represent the handler for getting the information about the result of executing the requested block
func (Server) StarknetGetStateUpdateByHash(c context.Context, p cmd.BlockHashOrTag) (cmd.StateUpdate, error) {
	return cmd.StateUpdate{}, nil
}

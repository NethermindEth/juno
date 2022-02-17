package rpc

import (
	"context"
	"fmt"
	cmd "github.com/NethermindEth/juno/cmd/starknet"
	pkg "github.com/NethermindEth/juno/pkg"
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
func (Server) StarknetCall(c context.Context, request cmd.FunctionCall, blockHash cmd.BlockHashOrTag) (cmd.ResultCall, error) {

	return []string{"Response", "of", "starknet_call"}, nil
}

// StarknetGetBlockByHash represent the handler for getting a block by its hash
func (Server) StarknetGetBlockByHash(c context.Context, blockHash cmd.BlockHashOrTag, requestedScope pkg.RequestedScope) (pkg.BlockResponse, error) {
	return pkg.BlockResponse{}, nil
}

// StarknetGetBlockByNumber represent the handler for getting a block by its number
func (Server) StarknetGetBlockByNumber(c context.Context, blockNumber uint64, requestedScope pkg.RequestedScope) (pkg.BlockResponse, error) {
	return pkg.BlockResponse{}, nil
}

// StarknetGetBlockTransactionCountByHash represent the handler for getting block transaction count by the blocks hash
func (Server) StarknetGetBlockTransactionCountByHash(c context.Context, blockHash cmd.BlockHashOrTag) (cmd.BlockTransactionCount, error) {
	return cmd.BlockTransactionCount{}, nil
}

// StarknetGetBlockTransactionCountByNumber Get the number of transactions in a block given a block number (height)
func (Server) StarknetGetBlockTransactionCountByNumber(c context.Context, blockNumber cmd.BlockNumberOrTag) (cmd.BlockTransactionCount, error) {
	return cmd.BlockTransactionCount{}, nil
}

// StarknetGetStateUpdateByHash represent the handler for getting the information about the result of executing the requested block
func (Server) StarknetGetStateUpdateByHash(c context.Context, blockHash cmd.BlockHashOrTag) (cmd.StateUpdate, error) {
	return cmd.StateUpdate{}, nil
}

// StarknetGetStorageAt Get the value of the storage at the given address and key
func (Server) StarknetGetStorageAt(c context.Context, contractAddress cmd.Address, key cmd.Felt, blockHash cmd.BlockHashOrTag) (cmd.Felt, error) {
	return "", nil
}

// StarknetGetTransactionByHash Get the details and status of a submitted transaction
func (Server) StarknetGetTransactionByHash(c context.Context, transactionHash cmd.TxnHash) (cmd.Txn, error) {
	return cmd.Txn{}, nil
}

// StarknetGetTransactionByBlockHashAndIndex Get the details of the transaction given by the identified block and index
// in that block. If no transaction is found, null is returned.
func (Server) StarknetGetTransactionByBlockHashAndIndex(c context.Context, blockHash cmd.BlockHashOrTag, index uint64) (cmd.Txn, error) {
	return cmd.Txn{}, nil
}

// StarknetGetTransactionByBlockNumberAndIndex Get the details of the transaction given by the identified block and index in that block. If no transaction is found, null is returned.
func (Server) StarknetGetTransactionByBlockNumberAndIndex(c context.Context, blockNumber cmd.BlockNumberOrTag, index uint64) (cmd.Txn, error) {
	return cmd.Txn{}, nil
}

// StarknetGetTransactionReceipt Get the transaction receipt by the transaction hash
func (Server) StarknetGetTransactionReceipt(c context.Context, transactionHash cmd.TxnHash) (cmd.TxnReceipt, error) {
	return cmd.TxnReceipt{}, nil
}

// StarknetGetCode Get the code of a specific contract
func (Server) StarknetGetCode(c context.Context, contractAddress cmd.Address) (cmd.CodeResult, error) {
	return cmd.CodeResult{}, nil
}

// StarknetBlockNumber Get the most recent accepted block number
func (Server) StarknetBlockNumber(c context.Context) (cmd.BlockNumber, error) {
	return 0, nil
}

// StarknetChainId Return the currently configured StarkNet chain id
func (Server) StarknetChainId(c context.Context) (cmd.ChainID, error) {
	return "Here the ChainIDg", nil
}

// StarknetPendingTransactions Returns the transactions in the transaction pool, recognized by this sequencer",
func (Server) StarknetPendingTransactions(c context.Context) ([]cmd.Txn, error) {
	return nil, nil
}

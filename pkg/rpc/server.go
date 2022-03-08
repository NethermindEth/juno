package rpc

import (
	"context"
	"net/http"

	"github.com/NethermindEth/juno/internal/log"
	types "github.com/NethermindEth/juno/pkg/types"
)

// Server represents the server structure
type Server struct {
	server http.Server
}

// HandlerRPC represents the struct that later we will apply reflection
// to call rpc methods.
type HandlerRPC struct{}

// NewServer creates a new server.
func NewServer(addr string) *Server {
	mr := NewHandlerJsonRpc(HandlerRPC{})
	http.Handle("/rpc", mr)
	return &Server{
		server: http.Server{Addr: addr, Handler: http.DefaultServeMux},
	}
}

// ListenAndServe listens on the TCP network and handles requests on
// incoming connections.
func (s *Server) ListenAndServe() error {
	// notest
	log.Default.Info("Listening for connections .... ")

	err := s.server.ListenAndServe()
	if err != nil {
		log.Default.With("Error", err).Error("Error listening for connections")
		return err
	}
	return nil
}

// Close gracefully shuts down the server.
func (s *Server) Close(ctx context.Context) {
	// notest
	select {
	case <-ctx.Done():
		err := s.server.Shutdown(ctx)
		if err != nil {
			log.Default.With("Error", err).Info("Exiting with error")
			return
		}
	default:
	}
}

// Echo represents the handler of "echo" rpc call, just reply with the same message
func (HandlerRPC) Echo(c context.Context, message string) (string, error) {
	return message, nil
}

// StarknetCall represents the handler of "starknet_call" rpc call
func (HandlerRPC) StarknetCall(c context.Context, request types.FunctionCall, blockHash types.BlockHashOrTag) (types.ResultCall, error) {
	return []string{"Response", "of", "starknet_call"}, nil
}

// StarknetGetBlockByHash represent the handler for getting a block by its hash
func (HandlerRPC) StarknetGetBlockByHash(c context.Context, blockHash types.BlockHashOrTag) (types.BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	return types.BlockResponse{
		BlockHash: string(blockHash),
	}, nil
}

// StarknetGetBlockByHashOpt represent the handler for getting a block by its hash
func (HandlerRPC) StarknetGetBlockByHashOpt(c context.Context, blockHash types.BlockHashOrTag, requestedScope types.RequestedScope) (types.BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	return types.BlockResponse{
		BlockHash:  string(blockHash),
		ParentHash: string(requestedScope),
	}, nil
}

//type bNumber string `json:"int,int,omitempty"`

// StarknetGetBlockByNumber represent the handler for getting a block by its number
func (HandlerRPC) StarknetGetBlockByNumber(c context.Context, blockNumber interface{}) (types.BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	log.Default.With("Block Number", blockNumber).Info("Calling StarknetGetBlockByNumber")
	return types.BlockResponse{}, nil
}

// StarknetGetBlockByNumberOpt represent the handler for getting a block by its number
func (HandlerRPC) StarknetGetBlockByNumberOpt(c context.Context, blockNumber interface{}, requestedScope types.RequestedScope) (types.BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	return types.BlockResponse{}, nil
}

// StarknetGetBlockTransactionCountByHash represent the handler for getting block transaction count by the blocks hash
func (HandlerRPC) StarknetGetBlockTransactionCountByHash(c context.Context, blockHash types.BlockHashOrTag) (types.BlockTransactionCount, error) {
	return types.BlockTransactionCount{}, nil
}

// StarknetGetBlockTransactionCountByNumber Get the number of transactions in a block given a block number (height)
func (HandlerRPC) StarknetGetBlockTransactionCountByNumber(c context.Context, blockNumber interface{}) (types.BlockTransactionCount, error) {
	return types.BlockTransactionCount{}, nil
}

// StarknetGetStateUpdateByHash represent the handler for getting the information about the result of executing the requested block
func (HandlerRPC) StarknetGetStateUpdateByHash(c context.Context, blockHash types.BlockHashOrTag) (types.StateUpdate, error) {
	return types.StateUpdate{}, nil
}

// StarknetGetStorageAt Get the value of the storage at the given address and key
func (HandlerRPC) StarknetGetStorageAt(c context.Context, contractAddress types.Address, key types.Felt, blockHash types.BlockHashOrTag) (types.Felt, error) {
	return "Storage", nil
}

// StarknetGetTransactionByHash Get the details and status of a submitted transaction
func (HandlerRPC) StarknetGetTransactionByHash(c context.Context, transactionHash types.TxnHash) (types.Txn, error) {
	return types.Txn{}, nil
}

// StarknetGetTransactionByBlockHashAndIndex Get the details of the transaction given by the identified block and index
// in that block. If no transaction is found, null is returned.
func (HandlerRPC) StarknetGetTransactionByBlockHashAndIndex(c context.Context, blockHash types.BlockHashOrTag, index uint64) (types.Txn, error) {
	return types.Txn{}, nil
}

// StarknetGetTransactionByBlockNumberAndIndex Get the details of the transaction given by the identified block and index in that block. If no transaction is found, null is returned.
func (HandlerRPC) StarknetGetTransactionByBlockNumberAndIndex(c context.Context, blockNumber types.BlockNumberOrTag, index uint64) (types.Txn, error) {
	return types.Txn{}, nil
}

// StarknetGetTransactionReceipt Get the transaction receipt by the transaction hash
func (HandlerRPC) StarknetGetTransactionReceipt(c context.Context, transactionHash types.TxnHash) (types.TxnReceipt, error) {
	return types.TxnReceipt{}, nil
}

// StarknetGetCode Get the code of a specific contract
func (HandlerRPC) StarknetGetCode(c context.Context, contractAddress types.Address) (types.CodeResult, error) {
	return types.CodeResult{}, nil
}

// StarknetBlockNumber Get the most recent accepted block number
func (HandlerRPC) StarknetBlockNumber(c context.Context) (types.BlockNumber, error) {
	return 0, nil
}

// StarknetChainId Return the currently configured StarkNet chain id
func (HandlerRPC) StarknetChainId(c context.Context) (types.ChainID, error) {
	return "Here the ChainID", nil
}

// StarknetPendingTransactions Returns the transactions in the transaction pool, recognized by this sequencer",
func (HandlerRPC) StarknetPendingTransactions(c context.Context) ([]types.Txn, error) {
	return nil, nil
}

// StarknetProtocolVersion Returns the current starknet protocol version identifier, as supported by this sequencer
func (HandlerRPC) StarknetProtocolVersion(c context.Context) (types.ProtocolVersion, error) {
	return "Here the Protocol Version", nil
}

// StarknetSyncing Returns an object about the sync status, or false if the node is not syncing
func (HandlerRPC) StarknetSyncing(c context.Context) (types.SyncStatus, error) {
	return types.SyncStatus{}, nil
}

// StarknetGetEvents Returns all event objects matching the conditions in the provided filter
func (HandlerRPC) StarknetGetEvents(c context.Context, r EventRequest) (EventResponse, error) {
	return EventResponse{}, nil
}

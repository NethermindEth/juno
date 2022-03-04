package rpc

import (
	"context"
	cmd "github.com/NethermindEth/juno/cmd/starknet"
	pkg "github.com/NethermindEth/juno/pkg"
	"go.uber.org/zap"
	"net/http"
)

// Server represent
type Server struct {
	server http.Server
}

// HandlerRPC represent
type HandlerRPC struct{}

// NewServer creates a new server
func NewServer(addr string) *Server {
	mr := NewMethodRepositoryWithMethods(HandlerRPC{})
	http.Handle("/rpc", mr)

	return &Server{
		server: http.Server{Addr: addr, Handler: http.DefaultServeMux},
	}
}

// ListenAndServe listen on the TCP network and handle requests on incoming connections
func (s *Server) ListenAndServe(l *zap.SugaredLogger) error {
	logger = l
	logger.Info("Listening for connections .... ")

	err := s.server.ListenAndServe()
	if err != nil {
		logger.With("Error", err).Error("Error listening for connections")
		return err
	}
	return nil
}

// Close gracefully shuts down the server
func (s *Server) Close(ctx context.Context) {
	select {
	case <-ctx.Done():
		err := s.server.Shutdown(ctx)
		if err != nil {
			return
		}
	default:
	}
}

// Echo represents the handler of "echo" rpc call, just reply with the same message
func (HandlerRPC) Echo(c context.Context, request Echo) (Echo, error) {
	return Echo{
		Message: request.Message,
	}, nil
}

// StarknetCall represents the handler of "starknet_call" rpc call
func (HandlerRPC) StarknetCall(c context.Context, request cmd.FunctionCall, blockHash cmd.BlockHashOrTag) (cmd.ResultCall, error) {
	return []string{"Response", "of", "starknet_call"}, nil
}

// StarknetGetBlockByHash represent the handler for getting a block by its hash
func (HandlerRPC) StarknetGetBlockByHash(c context.Context, blockHash cmd.BlockHashOrTag) (pkg.BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	return pkg.BlockResponse{
		BlockHash: string(blockHash),
	}, nil
}

// StarknetGetBlockByHashOpt represent the handler for getting a block by its hash
func (HandlerRPC) StarknetGetBlockByHashOpt(c context.Context, blockHash cmd.BlockHashOrTag, requestedScope pkg.RequestedScope) (pkg.BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	return pkg.BlockResponse{
		BlockHash:  string(blockHash),
		ParentHash: string(requestedScope),
	}, nil
}

// StarknetGetBlockByNumber represent the handler for getting a block by its number
func (HandlerRPC) StarknetGetBlockByNumber(c context.Context, blockNumber string) (pkg.BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	return pkg.BlockResponse{}, nil
}

// StarknetGetBlockByNumberOpt represent the handler for getting a block by its number
func (HandlerRPC) StarknetGetBlockByNumberOpt(c context.Context, blockNumber string, requestedScope pkg.RequestedScope) (pkg.BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	return pkg.BlockResponse{}, nil
}

// StarknetGetBlockTransactionCountByHash represent the handler for getting block transaction count by the blocks hash
func (HandlerRPC) StarknetGetBlockTransactionCountByHash(c context.Context, blockHash cmd.BlockHashOrTag) (cmd.BlockTransactionCount, error) {
	return cmd.BlockTransactionCount{}, nil
}

// StarknetGetBlockTransactionCountByNumber Get the number of transactions in a block given a block number (height)
func (HandlerRPC) StarknetGetBlockTransactionCountByNumber(c context.Context, blockNumber cmd.BlockNumberOrTag) (cmd.BlockTransactionCount, error) {
	return cmd.BlockTransactionCount{}, nil
}

// StarknetGetStateUpdateByHash represent the handler for getting the information about the result of executing the requested block
func (HandlerRPC) StarknetGetStateUpdateByHash(c context.Context, blockHash cmd.BlockHashOrTag) (cmd.StateUpdate, error) {
	return cmd.StateUpdate{}, nil
}

// StarknetGetStorageAt Get the value of the storage at the given address and key
func (HandlerRPC) StarknetGetStorageAt(c context.Context, contractAddress cmd.Address, key cmd.Felt, blockHash cmd.BlockHashOrTag) (cmd.Felt, error) {
	return "", nil
}

// StarknetGetTransactionByHash Get the details and status of a submitted transaction
func (HandlerRPC) StarknetGetTransactionByHash(c context.Context, transactionHash cmd.TxnHash) (cmd.Txn, error) {
	return cmd.Txn{}, nil
}

// StarknetGetTransactionByBlockHashAndIndex Get the details of the transaction given by the identified block and index
// in that block. If no transaction is found, null is returned.
func (HandlerRPC) StarknetGetTransactionByBlockHashAndIndex(c context.Context, blockHash cmd.BlockHashOrTag, index uint64) (cmd.Txn, error) {
	return cmd.Txn{}, nil
}

// StarknetGetTransactionByBlockNumberAndIndex Get the details of the transaction given by the identified block and index in that block. If no transaction is found, null is returned.
func (HandlerRPC) StarknetGetTransactionByBlockNumberAndIndex(c context.Context, blockNumber cmd.BlockNumberOrTag, index uint64) (cmd.Txn, error) {
	return cmd.Txn{}, nil
}

// StarknetGetTransactionReceipt Get the transaction receipt by the transaction hash
func (HandlerRPC) StarknetGetTransactionReceipt(c context.Context, transactionHash cmd.TxnHash) (cmd.TxnReceipt, error) {
	return cmd.TxnReceipt{}, nil
}

// StarknetGetCode Get the code of a specific contract
func (HandlerRPC) StarknetGetCode(c context.Context, contractAddress cmd.Address) (cmd.CodeResult, error) {
	return cmd.CodeResult{}, nil
}

// StarknetBlockNumber Get the most recent accepted block number
func (HandlerRPC) StarknetBlockNumber(c context.Context) (cmd.BlockNumber, error) {
	return 0, nil
}

// StarknetChainId Return the currently configured StarkNet chain id
func (HandlerRPC) StarknetChainId(c context.Context) (cmd.ChainID, error) {
	return "Here the ChainIDg", nil
}

// StarknetPendingTransactions Returns the transactions in the transaction pool, recognized by this sequencer",
func (HandlerRPC) StarknetPendingTransactions(c context.Context) ([]cmd.Txn, error) {
	return nil, nil
}

// StarknetProtocolVersion Returns the current starknet protocol version identifier, as supported by this sequencer
func (HandlerRPC) StarknetProtocolVersion(c context.Context) (cmd.ProtocolVersion, error) {
	return "Here the Protocol Version", nil
}

// StarknetSyncing Returns an object about the sync status, or false if the node is not syncing
func (HandlerRPC) StarknetSyncing(c context.Context) (cmd.SyncStatus, error) {
	return cmd.SyncStatus{}, nil
}

// StarknetGetEvents Returns all event objects matching the conditions in the provided filter
func (HandlerRPC) StarknetGetEvents(c context.Context, r EventRequest) (EventResponse, error) {
	return EventResponse{}, nil
}

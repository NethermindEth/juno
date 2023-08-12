//go:generate protoc --go_out=./ --proto_path=./ --go_opt=Mp2p/proto/requests.proto=./spec --go_opt=Mp2p/proto/transaction.proto=./spec --go_opt=Mp2p/proto/state.proto=./spec --go_opt=Mp2p/proto/snapshot.proto=./spec --go_opt=Mp2p/proto/receipt.proto=./spec --go_opt=Mp2p/proto/mempool.proto=./spec --go_opt=Mp2p/proto/event.proto=./spec --go_opt=Mp2p/proto/block.proto=./spec --go_opt=Mp2p/proto/common.proto=./spec p2p/proto/transaction.proto p2p/proto/state.proto p2p/proto/snapshot.proto p2p/proto/common.proto p2p/proto/block.proto p2p/proto/event.proto p2p/proto/receipt.proto p2p/proto/requests.proto
package starknet

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"google.golang.org/protobuf/proto"
)

type Handler struct {
	bcReader blockchain.Reader
	log      utils.Logger
}

func NewHandler(bcReader blockchain.Reader, log utils.Logger) *Handler {
	return &Handler{
		bcReader: bcReader,
		log:      log,
	}
}

// bufferPool caches unused buffer objects for later reuse.
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	return buffer
}

func (h *Handler) StreamHandler(stream network.Stream) {
	defer func() {
		if err := stream.Close(); err != nil {
			h.log.Debugw("Error closing stream", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		}
	}()

	buffer := getBuffer()
	defer bufferPool.Put(buffer)

	if _, err := buffer.ReadFrom(stream); err != nil {
		h.log.Debugw("Error reading from stream", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		return
	}

	var req spec.Request
	if err := proto.Unmarshal(buffer.Bytes(), &req); err != nil {
		h.log.Debugw("Error unmarshalling message", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		return
	}

	response, err := h.reqHandler(&req)
	if err != nil {
		h.log.Debugw("Error handling request", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err, "request", req.String())
		return
	}

	responseBytes, err := proto.Marshal(response)
	if err != nil {
		h.log.Debugw("Error marshalling response", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err, "response", response)
		return
	}

	if _, err = stream.Write(responseBytes); err != nil {
		h.log.Debugw("Error writing response", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		return
	}
}

func (h *Handler) reqHandler(req *spec.Request) (proto.Message, error) {
	switch typedReq := req.GetReq().(type) {
	case *spec.Request_GetBlocks:
		return h.HandleGetBlocks(typedReq.GetBlocks)
	case *spec.Request_GetSignatures:
		return h.HandleGetSignatures(typedReq.GetSignatures)
	case *spec.Request_GetEvents:
		return h.HandleGetEvents(typedReq.GetEvents)
	case *spec.Request_GetReceipts:
		return h.HandleGetReceipts(typedReq.GetReceipts)
	case *spec.Request_GetTransactions:
		return h.HandleGetTransactions(typedReq.GetTransactions)
	default:
		return nil, fmt.Errorf("unhandled request %T", typedReq)
	}
}

func (h *Handler) HandleGetBlocks(req *spec.GetBlocks) (*spec.GetBlocksResponse, error) {
	// todo: read from bcReader and adapt to p2p type
	return &spec.GetBlocksResponse{
		Blocks: []*spec.HeaderAndStateDiff{
			{
				Header: &spec.BlockHeader{
					State: &spec.Merkle{
						NLeaves: 251,
					},
				},
			},
		},
	}, nil
}

func (h *Handler) HandleGetSignatures(req *spec.GetSignatures) (*spec.Signatures, error) {
	// todo: read from bcReader and adapt to p2p type
	return &spec.Signatures{
		Id: req.Id,
	}, nil
}

func (h *Handler) HandleGetEvents(req *spec.GetEvents) (*spec.Events, error) {
	// todo: read from bcReader and adapt to p2p type
	magic := 44
	return &spec.Events{
		Events: make([]*spec.Event, magic),
	}, nil
}

func (h *Handler) HandleGetReceipts(req *spec.GetReceipts) (*spec.Receipts, error) {
	// todo: read from bcReader and adapt to p2p type
	magic := 37
	return &spec.Receipts{
		Receipts: make([]*spec.Receipt, magic),
	}, nil
}

func (h *Handler) HandleGetTransactions(req *spec.GetTransactions) (*spec.Transactions, error) {
	// todo: read from bcReader and adapt to p2p type
	magic := 1337
	return &spec.Transactions{
		Transactions: make([]*spec.Transaction, magic),
	}, nil
}

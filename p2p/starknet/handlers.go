//go:generate protoc --go_out=./ --proto_path=./ --go_opt=Mp2p/proto/transaction.proto=./spec --go_opt=Mp2p/proto/state.proto=./spec --go_opt=Mp2p/proto/snapshot.proto=./spec --go_opt=Mp2p/proto/receipt.proto=./spec --go_opt=Mp2p/proto/mempool.proto=./spec --go_opt=Mp2p/proto/event.proto=./spec --go_opt=Mp2p/proto/block.proto=./spec --go_opt=Mp2p/proto/common.proto=./spec p2p/proto/transaction.proto p2p/proto/state.proto p2p/proto/snapshot.proto p2p/proto/common.proto p2p/proto/block.proto p2p/proto/event.proto p2p/proto/receipt.proto
package starknet

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"google.golang.org/protobuf/encoding/protodelim"
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

func streamHandler[ReqT proto.Message](stream network.Stream,
	reqHandler func(req ReqT) (Stream[proto.Message], error), log utils.SimpleLogger,
) {
	defer func() {
		if err := stream.Close(); err != nil {
			log.Debugw("Error closing stream", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		}
	}()

	buffer := getBuffer()
	defer bufferPool.Put(buffer)

	if _, err := buffer.ReadFrom(stream); err != nil {
		log.Debugw("Error reading from stream", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		return
	}

	var zero ReqT
	req := zero.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(buffer.Bytes(), req); err != nil {
		log.Debugw("Error unmarshalling message", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		return
	}

	response, err := reqHandler(req.(ReqT))
	if err != nil {
		log.Debugw("Error handling request", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		return
	}

	for msg, valid := response(); valid; msg, valid = response() {
		if _, err := protodelim.MarshalTo(stream, msg); err != nil { // todo: figure out if we need buffered io here
			log.Debugw("Error writing response", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		}
	}
}

func (h *Handler) BlockHeadersHandler(stream network.Stream) {
	streamHandler[*spec.BlockHeadersRequest](stream, h.onBlockHeadersRequest, h.log)
}

func (h *Handler) BlockBodiesHandler(stream network.Stream) {
	streamHandler[*spec.BlockBodiesRequest](stream, h.onBlockBodiesRequest, h.log)
}

func (h *Handler) EventsHandler(stream network.Stream) {
	streamHandler[*spec.EventsRequest](stream, h.onEventsRequest, h.log)
}

func (h *Handler) ReceiptsHandler(stream network.Stream) {
	streamHandler[*spec.ReceiptsRequest](stream, h.onReceiptsRequest, h.log)
}

func (h *Handler) TransactionsHandler(stream network.Stream) {
	streamHandler[*spec.TransactionsRequest](stream, h.onTransactionsRequest, h.log)
}

func (h *Handler) onBlockHeadersRequest(req *spec.BlockHeadersRequest) (Stream[proto.Message], error) {
	it, err := h.newIterator(req.Iteration)
	if err != nil {
		return nil, err
	}

	fin := h.newFin(&spec.BlockHeadersResponse{
		Part: []*spec.BlockHeadersResponsePart{
			{
				HeaderMessage: &spec.BlockHeadersResponsePart_Fin{},
			},
		},
	})
	return func() (proto.Message, bool) {
		if !it.Valid() {
			return fin()
		}

		header, err := it.Header()
		if err != nil {
			h.log.Errorw("Failed to fetch header", "err", err)
			return fin()
		}
		it.Next()

		commitments, err := h.bcReader.BlockCommitmentsByNumber(header.Number)
		if err != nil {
			h.log.Errorw("Failed to fetch block commitments", "err", err)
			return fin()
		}

		return &spec.BlockHeadersResponse{
			Part: []*spec.BlockHeadersResponsePart{
				{
					HeaderMessage: &spec.BlockHeadersResponsePart_Header{
						Header: core2p2p.AdaptHeader(header, commitments),
					},
				},
				{
					HeaderMessage: &spec.BlockHeadersResponsePart_Signatures{
						Signatures: &spec.Signatures{
							Block:      core2p2p.AdaptBlockID(header),
							Signatures: utils.Map(header.Signatures, core2p2p.AdaptSignature),
						},
					},
				},
			},
		}, true
	}, nil
}

func (h *Handler) onBlockBodiesRequest(req *spec.BlockBodiesRequest) (Stream[proto.Message], error) {
	// todo: read from bcReader and adapt to p2p type
	count := uint64(0)
	return func() (proto.Message, bool) {
		if count > 3 {
			return nil, false
		}
		count++
		return &spec.BlockBodiesResponse{
			Id: &spec.BlockID{
				Number: count - 1,
			},
		}, true
	}, nil
}

func (h *Handler) onEventsRequest(req *spec.EventsRequest) (Stream[proto.Message], error) {
	it, err := h.newIterator(req.Iteration)
	if err != nil {
		return nil, err
	}

	fin := h.newFin(&spec.EventsResponse{
		Responses: &spec.EventsResponse_Fin{},
	})
	return func() (proto.Message, bool) {
		if !it.Valid() {
			return fin()
		}

		block, err := it.Block()
		if err != nil {
			h.log.Errorw("Failed to fetch block", "err", err)
			return fin()
		}
		it.Next()

		events := make([]*spec.Event, 0, len(block.Receipts))
		for _, receipt := range block.Receipts {
			for _, event := range receipt.Events {
				events = append(events, core2p2p.AdaptEvent(event))
			}
		}

		return &spec.EventsResponse{
			Id: core2p2p.AdaptBlockID(block.Header),
			Responses: &spec.EventsResponse_Events{
				Events: &spec.Events{
					Items: events,
				},
			},
		}, true
	}, nil
}

func (h *Handler) onReceiptsRequest(req *spec.ReceiptsRequest) (Stream[proto.Message], error) {
	blockchainIt, err := h.newIterator(req.Iteration)
	if err != nil {
		return nil, err
	}

	fin := h.newFin(&spec.ReceiptsResponse{Responses: &spec.ReceiptsResponse_Fin{}})

	return func() (proto.Message, bool) {
		if !blockchainIt.Valid() {
			return fin()
		}

		b, err := blockchainIt.Block()
		if err != nil {
			h.log.Errorw("Failed to fetch block", "block number", b.Number, "err", err)
			return fin()
		}
		blockchainIt.Next()

		receipts := make([]*spec.Receipt, len(b.Receipts))
		for i := 0; i < len(b.Receipts); i++ {
			receipts[i] = core2p2p.AdaptReceipt(b.Receipts[i], b.Transactions[i])
		}

		rs := &spec.Receipts{Items: receipts}
		res := &spec.ReceiptsResponse{
			Id:        core2p2p.AdaptBlockID(b.Header),
			Responses: &spec.ReceiptsResponse_Receipts{Receipts: rs},
		}

		return res, true
	}, nil
}

func (h *Handler) onTransactionsRequest(req *spec.TransactionsRequest) (Stream[proto.Message], error) {
	it, err := h.newIterator(req.Iteration)
	if err != nil {
		return nil, err
	}

	fin := h.newFin(&spec.TransactionsResponse{
		Responses: &spec.TransactionsResponse_Fin{},
	})

	return func() (proto.Message, bool) {
		if !it.Valid() {
			return fin()
		}

		block, err := it.Block()
		if err != nil {
			h.log.Errorw("Iterator failure", "err", err)
			return fin()
		}
		it.Next()

		return &spec.TransactionsResponse{
			Id: core2p2p.AdaptBlockID(block.Header),
			Responses: &spec.TransactionsResponse_Transactions{
				Transactions: &spec.Transactions{
					Items: utils.Map(block.Transactions, core2p2p.AdaptTransaction),
				},
			},
		}, true
	}, nil
}

func (h *Handler) newIterator(it *spec.Iteration) (*iterator, error) {
	forward := it.Direction == spec.Iteration_Forward

	switch v := it.Start.(type) {
	case *spec.Iteration_BlockNumber:
		return newIteratorByNumber(h.bcReader, v.BlockNumber, it.Limit, it.Step, forward)
	case *spec.Iteration_Header:
		return newIteratorByHash(h.bcReader, p2p2core.AdaptHash(v.Header), it.Limit, it.Step, forward)
	default:
		return nil, fmt.Errorf("unsupported iteration start type %T", v)
	}
}

func (h *Handler) newFin(finMsg proto.Message) func() (proto.Message, bool) {
	var finSent bool

	return func() (proto.Message, bool) {
		if finSent {
			return nil, false
		}
		finSent = true

		return finMsg, true
	}
}

//go:generate protoc --go_out=./ --proto_path=./ --go_opt=Mp2p/proto/transaction.proto=./spec --go_opt=Mp2p/proto/state.proto=./spec --go_opt=Mp2p/proto/snapshot.proto=./spec --go_opt=Mp2p/proto/receipt.proto=./spec --go_opt=Mp2p/proto/mempool.proto=./spec --go_opt=Mp2p/proto/event.proto=./spec --go_opt=Mp2p/proto/block.proto=./spec --go_opt=Mp2p/proto/common.proto=./spec p2p/proto/transaction.proto p2p/proto/state.proto p2p/proto/snapshot.proto p2p/proto/common.proto p2p/proto/block.proto p2p/proto/event.proto p2p/proto/receipt.proto
package starknet

import (
	"bytes"
	"sync"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
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
	// todo: read from bcReader and adapt to p2p type
	count := uint64(0)
	return func() (proto.Message, bool) {
		if count > 3 {
			return nil, false
		}
		count++
		return &spec.BlockHeadersResponse{
			Part: []*spec.BlockHeadersResponsePart{
				{
					HeaderMessage: &spec.BlockHeadersResponsePart_Header{
						Header: &spec.BlockHeader{
							Number: count - 1,
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

	return func() (proto.Message, bool) {
		if it.Valid() {
			return nil, false
		}

		block, err := it.Block()
		if err != nil {
			h.log.Errorw("Failed to fetch block", "err", err)
			return nil, false
		}
		it.Next()

		var events []*core.Event
		for _, item := range block.Receipts {
			events = append(events, item.Events...)
		}

		return &spec.EventsResponse{
			Id: core2p2p.AdaptBlockID(block.Header),
			Responses: &spec.EventsResponse_Events{
				Events: &spec.Events{
					Items: utils.Map(events, core2p2p.AdaptEvent),
				},
			},
		}, true
	}, nil
}

func (h *Handler) onReceiptsRequest(req *spec.ReceiptsRequest) (Stream[proto.Message], error) {
	// todo: read from bcReader and adapt to p2p type
	count := uint64(0)
	return func() (proto.Message, bool) {
		if count > 3 {
			return nil, false
		}
		count++
		return &spec.ReceiptsResponse{
			Id: &spec.BlockID{
				Number: count - 1,
			},
		}, true
	}, nil
}

func (h *Handler) onTransactionsRequest(req *spec.TransactionsRequest) (Stream[proto.Message], error) {
	// todo: read from bcReader and adapt to p2p type
	count := uint64(0)
	return func() (proto.Message, bool) {
		if count > 3 {
			return nil, false
		}
		count++
		return &spec.TransactionsResponse{
			Id: &spec.BlockID{
				Number: count - 1,
			},
		}, true
	}, nil
}

func (h *Handler) newIterator(it *spec.Iteration) (*iterator, error) {
	forward := it.Direction == spec.Iteration_Forward
	return newIterator(h.bcReader, it.GetBlockNumber(), it.Limit, it.Step, forward)
}

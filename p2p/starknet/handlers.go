//go:generate protoc --go_out=./ --proto_path=./ --go_opt=Mp2p/proto/transaction.proto=./spec --go_opt=Mp2p/proto/state.proto=./spec --go_opt=Mp2p/proto/snapshot.proto=./spec --go_opt=Mp2p/proto/receipt.proto=./spec --go_opt=Mp2p/proto/mempool.proto=./spec --go_opt=Mp2p/proto/event.proto=./spec --go_opt=Mp2p/proto/block.proto=./spec --go_opt=Mp2p/proto/common.proto=./spec p2p/proto/transaction.proto p2p/proto/state.proto p2p/proto/snapshot.proto p2p/proto/common.proto p2p/proto/block.proto p2p/proto/event.proto p2p/proto/receipt.proto
package starknet

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/utils/iter"
	"github.com/libp2p/go-libp2p/core/network"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"
)

type Handler struct {
	bcReader blockchain.Reader
	log      utils.SimpleLogger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewHandler(bcReader blockchain.Reader, log utils.SimpleLogger) *Handler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Handler{
		bcReader: bcReader,
		log:      log,
		ctx:      ctx,
		cancel:   cancel,
		wg:       sync.WaitGroup{},
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

func streamHandler[ReqT proto.Message](ctx context.Context, wg *sync.WaitGroup,
	stream network.Stream, reqHandler func(req ReqT) (iter.Seq[proto.Message], error), log utils.SimpleLogger,
) {
	wg.Add(1)
	defer wg.Done()

	defer func() {
		if err := stream.Close(); err != nil {
			log.Debugw("Error closing stream", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		}
	}()

	buffer := getBuffer()
	defer bufferPool.Put(buffer)

	// todo add limit reader
	// todo add read timeout
	if _, err := buffer.ReadFrom(stream); err != nil {
		log.Debugw("Error reading from stream", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		return
	}

	// todo double check that wrong proto request is not possible
	var zero ReqT
	req := zero.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(buffer.Bytes(), req); err != nil {
		log.Debugw("Error unmarshalling message", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		return
	}

	responseIterator, err := reqHandler(req.(ReqT))
	if err != nil {
		// todo report error to client?
		log.Debugw("Error handling request", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
		return
	}

	// todo add write timeout
	responseIterator(func(msg proto.Message) bool {
		if ctx.Err() != nil {
			return false
		}

		if _, err := protodelim.MarshalTo(stream, msg); err != nil { // todo: figure out if we need buffered io here
			log.Debugw("Error writing response", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
			return false
		}

		return true
	})
}

func (h *Handler) BlockHeadersHandler(stream network.Stream) {
	streamHandler[*spec.BlockHeadersRequest](h.ctx, &h.wg, stream, h.onBlockHeadersRequest, h.log)
}

//func (h *Handler) BlockBodiesHandler(stream network.Stream) {
//	streamHandler[*spec.BlockBodiesRequest](h.ctx, &h.wg, stream, h.onBlockBodiesRequest, h.log)
//}

func (h *Handler) EventsHandler(stream network.Stream) {
	streamHandler[*spec.EventsRequest](h.ctx, &h.wg, stream, h.onEventsRequest, h.log)
}

func (h *Handler) ReceiptsHandler(stream network.Stream) {
	streamHandler[*spec.ReceiptsRequest](h.ctx, &h.wg, stream, h.onReceiptsRequest, h.log)
}

func (h *Handler) TransactionsHandler(stream network.Stream) {
	streamHandler[*spec.TransactionsRequest](h.ctx, &h.wg, stream, h.onTransactionsRequest, h.log)
}

//func (h *Handler) CurrentBlockHeaderHandler(stream network.Stream) {
//	streamHandler[*spec.CurrentBlockHeaderRequest](h.ctx, &h.wg, stream, h.onCurrentBlockHeaderRequest, h.log)
//}

//func (h *Handler) onCurrentBlockHeaderRequest(*spec.CurrentBlockHeaderRequest) (iter.Seq[proto.Message], error) {
//	curHeight, err := h.bcReader.Height()
//	if err != nil {
//		return nil, err
//	}
//
//	return h.onBlockHeadersRequest(&spec.BlockHeadersRequest{
//		Iteration: &spec.Iteration{
//			Start: &spec.Iteration_BlockNumber{
//				BlockNumber: curHeight,
//			},
//			Direction: spec.Iteration_Forward,
//			Limit:     1,
//			Step:      1,
//		},
//	})
//}

func (h *Handler) onBlockHeadersRequest(req *spec.BlockHeadersRequest) (iter.Seq[proto.Message], error) {
	finMsg := &spec.BlockHeadersResponse{
		HeaderMessage: &spec.BlockHeadersResponse_Fin{},
	}

	return h.processIterationRequest(req.Iteration, finMsg, func(it blockDataAccessor) (proto.Message, error) {
		header, err := it.Header()
		if err != nil {
			return nil, err
		}

		h.log.Debugw("Created Header Iterator", "blockNumber", header.Number)

		commitments, err := h.bcReader.BlockCommitmentsByNumber(header.Number)
		if err != nil {
			return nil, err
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
		}, nil
	})
}

/*
func (h *Handler) onBlockBodiesRequest(req *spec.BlockBodiesRequest) (iter.Seq[proto.Message], error) {
	it, err := h.newIterator(req.Iteration)
	if err != nil {
		return nil, err
	}

	fin := newFin(&spec.BlockBodiesResponse{
		BodyMessage: &spec.BlockBodiesResponse_Fin{},
	})

	return func(yield func(proto.Message) bool) {
	outerLoop:
		for it.Valid() {
			header, err := it.Header()
			if err != nil {
				h.log.Debugw("Failed to fetch header", "blockNumber", it.BlockNumber(), "err", err)
				break
			}

			h.log.Debugw("Creating Block Body Iterator", "blockNumber", header.Number)
			bodyIterator, err := newBlockBodyIterator(h.bcReader, header, h.log)
			if err != nil {
				h.log.Debugw("Failed to create block body iterator", "blockNumber", it.BlockNumber(), "err", err)
				break
			}

			for bodyIterator.hasNext() {
				msg, ok := bodyIterator.next()
				if !ok {
					break
				}

				if !yield(msg) {
					break outerLoop
				}
			}

			it.Next()
		}

		if finMs, ok := fin(); ok {
			yield(finMs)
		}
		yield(finMsg)
	}, nil
}
*/

func (h *Handler) onEventsRequest(req *spec.EventsRequest) (iter.Seq[proto.Message], error) {
	finMsg := &spec.EventsResponse{
		EventMessage: &spec.EventsResponse_Fin{},
	}
	return h.processIterationRequest(req.Iteration, finMsg, func(it blockDataAccessor) (proto.Message, error) {
		block, err := it.Block()
		if err != nil {
			return nil, err
		}

		events := make([]*spec.Event, 0, len(block.Receipts))
		for _, receipt := range block.Receipts {
			for _, event := range receipt.Events {
				events = append(events, core2p2p.AdaptEvent(event, receipt.TransactionHash))
			}
		}

		return &spec.EventsResponse{
			Id: core2p2p.AdaptBlockID(block.Header),
			Responses: &spec.EventsResponse_Events{
				Events: &spec.Events{
					Items: events,
				},
			},
		}, nil
	})
}

func (h *Handler) onReceiptsRequest(req *spec.ReceiptsRequest) (iter.Seq[proto.Message], error) {
	finMsg := &spec.ReceiptsResponse{Responses: &spec.ReceiptsResponse_Fin{}}
	return h.processIterationRequest(req.Iteration, finMsg, func(it blockDataAccessor) (proto.Message, error) {
		block, err := it.Block()
		if err != nil {
			return nil, err
		}

		receipts := make([]*spec.Receipt, len(block.Receipts))
		for i := 0; i < len(block.Receipts); i++ {
			receipts[i] = core2p2p.AdaptReceipt(block.Receipts[i], block.Transactions[i])
		}

		return &spec.ReceiptsResponse{
			Id: core2p2p.AdaptBlockID(block.Header),
			Responses: &spec.ReceiptsResponse_Receipts{
				Receipts: &spec.Receipts{Items: receipts},
			},
		}, nil
	})
}

func (h *Handler) onTransactionsRequest(req *spec.TransactionsRequest) (iter.Seq[proto.Message], error) {
	finMsg := &spec.TransactionsResponse{
		Responses: &spec.TransactionsResponse_Fin{},
	}
	return h.processIterationRequest(req.Iteration, finMsg, func(it blockDataAccessor) (proto.Message, error) {
		block, err := it.Block()
		if err != nil {
			return nil, err
		}

		return &spec.TransactionsResponse{
			Id: core2p2p.AdaptBlockID(block.Header),
			Responses: &spec.TransactionsResponse_Transactions{
				Transactions: &spec.Transactions{
					Items: utils.Map(block.Transactions, core2p2p.AdaptTransaction),
				},
			},
		}, nil
	})
}

// blockDataAccessor provides access to either entire block or header
// for current iteration
type blockDataAccessor interface {
	Block() (*core.Block, error)
	Header() (*core.Header, error)
}

// iterationProcessor is an alias for a function that will generate corresponding data
// given block data for current iteration through blockDataAccessor
type iterationProcessor = func(it blockDataAccessor) (proto.Message, error)

// processIterationRequest is helper function that simplifies data processing for provided spec.Iteration object
// caller usually passes iteration object from received request, finMsg as final message to a peer
// and iterationProcessor function that will generate response for each iteration
func (h *Handler) processIterationRequest(iteration *spec.Iteration, finMsg proto.Message,
	getMsg iterationProcessor,
) (iter.Seq[proto.Message], error) {
	it, err := h.newIterator(iteration)
	if err != nil {
		return nil, err
	}

	type yieldFunc = func(proto.Message) bool
	return func(yield yieldFunc) {
		// while iterator is valid
		for it.Valid() {
			// pass it to handler function (some might be interested in header, others in entire block)
			msg, err := getMsg(it)
			if err != nil {
				h.log.Errorw("Failed to generate data", "blockNumber", it.BlockNumber(), "err", err)
				break
			}

			// push generated msg to caller
			if !yield(msg) {
				// if caller is not interested in remaining data (example: connection to a peer is closed) exit
				// note that in this case we won't send finMsg
				return
			}

			it.Next()
		}

		// either we iterated over whole sequence or reached break statement in loop above
		// note that return value of yield is not checked because this is the last message anyway
		yield(finMsg)
	}, nil
}

func (h *Handler) newIterator(it *spec.Iteration) (*iterator, error) {
	forward := it.Direction == spec.Iteration_Forward
	// todo restrict limit max value ?
	switch v := it.Start.(type) {
	case *spec.Iteration_BlockNumber:
		return newIteratorByNumber(h.bcReader, v.BlockNumber, it.Limit, it.Step, forward)
	case *spec.Iteration_Header:
		return newIteratorByHash(h.bcReader, p2p2core.AdaptHash(v.Header), it.Limit, it.Step, forward)
	default:
		return nil, fmt.Errorf("unsupported iteration start type %T", v)
	}
}

func (h *Handler) Close() {
	fmt.Println("Canceling")
	h.cancel()
	fmt.Println("Waiting")
	h.wg.Wait()
	fmt.Println("Done")
}

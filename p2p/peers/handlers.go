package peers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"sync"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	syncclass "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/class"
	synccommon "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/event"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/header"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/state"
	synctransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/transaction"
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

	for msg := range responseIterator {
		if ctx.Err() != nil {
			break
		}

		// todo add write timeout
		if _, err := protodelim.MarshalTo(stream, msg); err != nil { // todo: figure out if we need buffered io here
			log.Debugw("Error writing response", "peer", stream.ID(), "protocol", stream.Protocol(), "err", err)
			break
		}
	}
}

func (h *Handler) HeadersHandler(stream network.Stream) {
	streamHandler(h.ctx, &h.wg, stream, h.onHeadersRequest, h.log)
}

func (h *Handler) EventsHandler(stream network.Stream) {
	streamHandler(h.ctx, &h.wg, stream, h.onEventsRequest, h.log)
}

func (h *Handler) TransactionsHandler(stream network.Stream) {
	streamHandler(h.ctx, &h.wg, stream, h.onTransactionsRequest, h.log)
}

func (h *Handler) ClassesHandler(stream network.Stream) {
	streamHandler(h.ctx, &h.wg, stream, h.onClassesRequest, h.log)
}

func (h *Handler) StateDiffHandler(stream network.Stream) {
	streamHandler(h.ctx, &h.wg, stream, h.onStateDiffRequest, h.log)
}

func (h *Handler) onHeadersRequest(req *header.BlockHeadersRequest) (iter.Seq[proto.Message], error) {
	finMsg := &header.BlockHeadersResponse{
		HeaderMessage: &header.BlockHeadersResponse_Fin{},
	}

	return h.processIterationRequest(req.Iteration, finMsg, func(it blockDataAccessor) (proto.Message, error) {
		blockHeader, err := it.Header()
		if err != nil {
			return nil, err
		}

		h.log.Debugw("Created Header Iterator", "blockNumber", blockHeader.Number)

		stateUpdate, err := h.bcReader.StateUpdateByNumber(blockHeader.Number)
		if err != nil {
			return nil, err
		}

		blockVer, err := core.ParseBlockVersion(blockHeader.ProtocolVersion)
		if err != nil {
			return nil, err
		}

		var commitments *core.BlockCommitments
		if blockVer.LessThan(core.Ver0_13_2) {
			block, err := it.Block()
			if err != nil {
				return nil, err
			}
			_, commitments, err = core.Post0132Hash(block, stateUpdate.StateDiff)
			if err != nil {
				return nil, err
			}
		} else {
			commitments, err = h.bcReader.BlockCommitmentsByNumber(blockHeader.Number)
			if err != nil {
				return nil, err
			}
		}

		return &header.BlockHeadersResponse{
			HeaderMessage: &header.BlockHeadersResponse_Header{
				Header: core2p2p.AdaptHeader(blockHeader, commitments, stateUpdate.StateDiff.Hash(),
					stateUpdate.StateDiff.Length()),
			},
		}, nil
	})
}

func (h *Handler) onEventsRequest(req *event.EventsRequest) (iter.Seq[proto.Message], error) {
	finMsg := &event.EventsResponse{
		EventMessage: &event.EventsResponse_Fin{},
	}
	return h.processIterationRequestMulti(req.Iteration, finMsg, func(it blockDataAccessor) ([]proto.Message, error) {
		block, err := it.Block()
		if err != nil {
			return nil, err
		}

		responses := make([]proto.Message, 0, len(block.Receipts))
		for _, receipt := range block.Receipts {
			for _, e := range receipt.Events {
				responses = append(responses, &event.EventsResponse{
					EventMessage: &event.EventsResponse_Event{
						Event: core2p2p.AdaptEvent(e, receipt.TransactionHash),
					},
				})
			}
		}

		return responses, nil
	})
}

func (h *Handler) onTransactionsRequest(req *synctransaction.TransactionsRequest) (iter.Seq[proto.Message], error) {
	finMsg := &synctransaction.TransactionsResponse{
		TransactionMessage: &synctransaction.TransactionsResponse_Fin{},
	}
	return h.processIterationRequestMulti(req.Iteration, finMsg, func(it blockDataAccessor) ([]proto.Message, error) {
		block, err := it.Block()
		if err != nil {
			return nil, err
		}

		responses := make([]proto.Message, len(block.Transactions))
		for i, tx := range block.Transactions {
			receipt := block.Receipts[i]

			responses[i] = &synctransaction.TransactionsResponse{
				TransactionMessage: &synctransaction.TransactionsResponse_TransactionWithReceipt{
					TransactionWithReceipt: &synctransaction.TransactionWithReceipt{
						Transaction: core2p2p.AdaptTransaction(tx),
						Receipt:     core2p2p.AdaptReceipt(receipt, tx),
					},
				},
			}
		}

		return responses, nil
	})
}

//nolint:gocyclo
func (h *Handler) onStateDiffRequest(req *state.StateDiffsRequest) (iter.Seq[proto.Message], error) {
	finMsg := &state.StateDiffsResponse{
		StateDiffMessage: &state.StateDiffsResponse_Fin{},
	}
	return h.processIterationRequestMulti(req.Iteration, finMsg, func(it blockDataAccessor) ([]proto.Message, error) {
		block, err := it.Block()
		if err != nil {
			return nil, err
		}
		blockNumber := block.Number

		stateUpdate, err := h.bcReader.StateUpdateByNumber(blockNumber)
		if err != nil {
			return nil, err
		}
		diff := stateUpdate.StateDiff

		type contractDiff struct {
			address      *felt.Felt
			storageDiffs map[felt.Felt]*felt.Felt
			nonce        *felt.Felt
			classHash    *felt.Felt // set only if contract deployed or replaced
		}
		modifiedContracts := make(map[felt.Felt]*contractDiff)

		initContractDiff := func(addr *felt.Felt) *contractDiff {
			return &contractDiff{address: addr}
		}
		updateModifiedContracts := func(addr felt.Felt, f func(*contractDiff)) error {
			cDiff, ok := modifiedContracts[addr]
			if !ok {
				cDiff = initContractDiff(&addr)
				if err != nil {
					return err
				}
				modifiedContracts[addr] = cDiff
			}

			f(cDiff)
			return nil
		}

		for addr, n := range diff.Nonces {
			err = updateModifiedContracts(addr, func(diff *contractDiff) {
				diff.nonce = n
			})
			if err != nil {
				return nil, err
			}
		}

		for addr, sDiff := range diff.StorageDiffs {
			err = updateModifiedContracts(addr, func(diff *contractDiff) {
				diff.storageDiffs = sDiff
			})
			if err != nil {
				return nil, err
			}
		}

		for addr, classHash := range diff.DeployedContracts {
			classHashCopy := classHash
			err = updateModifiedContracts(addr, func(diff *contractDiff) {
				diff.classHash = classHashCopy
			})
			if err != nil {
				return nil, err
			}
		}

		for addr, classHash := range diff.ReplacedClasses {
			classHashCopy := classHash
			err = updateModifiedContracts(addr, func(diff *contractDiff) {
				diff.classHash = classHashCopy
			})
			if err != nil {
				return nil, err
			}
		}

		var responses []proto.Message
		for _, c := range modifiedContracts {
			responses = append(responses, &state.StateDiffsResponse{
				StateDiffMessage: &state.StateDiffsResponse_ContractDiff{
					ContractDiff: core2p2p.AdaptContractDiff(c.address, c.nonce, c.classHash, c.storageDiffs),
				},
			})
		}

		for _, classHash := range diff.DeclaredV0Classes {
			responses = append(responses, &state.StateDiffsResponse{
				StateDiffMessage: &state.StateDiffsResponse_DeclaredClass{
					DeclaredClass: &state.DeclaredClass{
						ClassHash:         core2p2p.AdaptHash(classHash),
						CompiledClassHash: nil, // for cairo0 it's nil
					},
				},
			})
		}
		for classHash, compiledHash := range diff.DeclaredV1Classes {
			responses = append(responses, &state.StateDiffsResponse{
				StateDiffMessage: &state.StateDiffsResponse_DeclaredClass{
					DeclaredClass: &state.DeclaredClass{
						ClassHash:         core2p2p.AdaptHash(&classHash),
						CompiledClassHash: core2p2p.AdaptHash(compiledHash),
					},
				},
			})
		}

		return responses, nil
	})
}

func (h *Handler) onClassesRequest(req *syncclass.ClassesRequest) (iter.Seq[proto.Message], error) {
	finMsg := &syncclass.ClassesResponse{
		ClassMessage: &syncclass.ClassesResponse_Fin{},
	}
	return h.processIterationRequestMulti(req.Iteration, finMsg, func(it blockDataAccessor) ([]proto.Message, error) {
		block, err := it.Block()
		if err != nil {
			return nil, err
		}
		blockNumber := block.Number

		stateUpdate, err := h.bcReader.StateUpdateByNumber(blockNumber)
		if err != nil {
			return nil, err
		}

		stateReader, closer, err := h.bcReader.StateAtBlockNumber(blockNumber)
		if err != nil {
			return nil, err
		}
		defer func() {
			if closeErr := closer(); closeErr != nil {
				h.log.Errorw("Failed to close state reader", "err", closeErr)
			}
		}()

		stateDiff := stateUpdate.StateDiff

		var responses []proto.Message
		for _, hash := range stateDiff.DeclaredV0Classes {
			cls, err := stateReader.Class(hash)
			if err != nil {
				return nil, err
			}

			responses = append(responses, &syncclass.ClassesResponse{
				ClassMessage: &syncclass.ClassesResponse_Class{
					Class: core2p2p.AdaptClass(cls.Class),
				},
			})
		}
		for classHash := range stateDiff.DeclaredV1Classes {
			cls, err := stateReader.Class(&classHash)
			if err != nil {
				return nil, err
			}

			responses = append(responses, &syncclass.ClassesResponse{
				ClassMessage: &syncclass.ClassesResponse_Class{
					Class: core2p2p.AdaptClass(cls.Class),
				},
			})
		}

		return responses, nil
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
func (h *Handler) processIterationRequest(iteration *synccommon.Iteration, finMsg proto.Message,
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
				if !errors.Is(err, db.ErrKeyNotFound) {
					h.log.Errorw("Failed to generate data", "blockNumber", it.BlockNumber(), "err", err)
				}
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

type iterationProcessorMulti = func(it blockDataAccessor) ([]proto.Message, error)

func (h *Handler) processIterationRequestMulti(iteration *synccommon.Iteration, finMsg proto.Message,
	getMsg iterationProcessorMulti,
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
			messages, err := getMsg(it)
			if err != nil {
				if !errors.Is(err, db.ErrKeyNotFound) {
					h.log.Errorw("Failed to generate data", "blockNumber", it.BlockNumber(), "err", err)
				}
				break
			}

			for _, msg := range messages {
				// push generated msg to caller
				if !yield(msg) {
					// if caller is not interested in remaining data (example: connection to a peer is closed) exit
					// note that in this case we won't send finMsg
					return
				}
			}

			it.Next()
		}

		// either we iterated over whole sequence or reached break statement in loop above
		// note that return value of yield is not checked because this is the last message anyway
		yield(finMsg)
	}, nil
}

func (h *Handler) newIterator(it *synccommon.Iteration) (*iterator, error) {
	forward := it.Direction == synccommon.Iteration_Forward
	// todo restrict limit max value ?
	switch v := it.Start.(type) {
	case *synccommon.Iteration_BlockNumber:
		return newIteratorByNumber(h.bcReader, v.BlockNumber, it.Limit, it.Step, forward)
	case *synccommon.Iteration_Header:
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

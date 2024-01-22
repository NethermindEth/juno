package p2p

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/p2p/starknet"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/utils/pipeline"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const maxBlocks = 100

type syncService struct {
	host     host.Host
	network  utils.Network
	bootNode peer.ID
	client   *starknet.Client // todo: merge all the functionality of Client with p2p SyncService

	blockchain *blockchain.Blockchain
	log        utils.SimpleLogger
}

func newSyncService(bc *blockchain.Blockchain, h host.Host, bootNode peer.ID, network utils.Network,
	log utils.SimpleLogger,
) *syncService {
	return &syncService{
		host:       h,
		network:    network,
		blockchain: bc,
		log:        log,
		bootNode:   bootNode,
	}
}

func (s *syncService) bootNodeHeight(ctx context.Context) (uint64, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, s.bootNode, pids...)
	}, s.network, s.log)

	headersIt, err := c.RequestCurrentBlockHeader(ctx, &spec.CurrentBlockHeaderRequest{})
	if err != nil {
		return 0, err
	}

	var header *spec.BlockHeader
	for res, valid := headersIt(); valid; res, valid = headersIt() {
		for _, part := range res.GetPart() {
			if _, ok := part.HeaderMessage.(*spec.BlockHeadersResponsePart_Header); ok {
				header = part.GetHeader()
			}
		}
	}

	return header.Number, nil
}

func (s *syncService) start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.client = starknet.NewClient(s.randomPeerStream, s.network, s.log)

	var bootNodeHeight uint64
	for i := 0; ; i++ {
		if err := ctx.Err(); err != nil {
			break
		}

		fmt.Println("Continuous iteration", "i", i)

		ctx, cancelIteration := context.WithCancel(ctx)

		var err error
		bootNodeHeight, err = s.bootNodeHeight(ctx)
		if err != nil {
			s.logError("Failed to get boot node height", err)
			cancelIteration()
			continue
		}

		var nextHeight uint64
		if curHeight, err := s.blockchain.Height(); err == nil { //nolint:govet
			nextHeight = curHeight + 1
		} else if !errors.Is(db.ErrKeyNotFound, err) {
			s.log.Errorw("Failed to get current height", "err", err)
		}

		blockBehind := bootNodeHeight - (nextHeight - 1)
		if blockBehind <= 0 {
			s.log.Infow("Bootnode height is the same as local height, retrying in 30s")
			time.Sleep(30 * time.Second)
			cancelIteration()
			continue
		}

		s.log.Infow("Start Pipeline", "Bootnode height", bootNodeHeight, "Current height", nextHeight-1)
		s.log.Infow("Fetching blocks", "Start", nextHeight, "End", nextHeight+min(blockBehind, maxBlocks))

		commonIt := s.createIterator(nextHeight, min(blockBehind, maxBlocks))
		headersAndSigsCh, err := s.genHeadersAndSigs(ctx, commonIt)
		if err != nil {
			s.logError("Failed to get block headers parts", err)
			cancelIteration()
			continue
		}

		blockBodiesCh, err := s.genBlockBodies(ctx, commonIt)
		if err != nil {
			s.logError("Failed to get block bodies", err)
			cancelIteration()
			continue
		}

		txsCh, err := s.genTransactions(ctx, commonIt)
		if err != nil {
			s.logError("Failed to get transactions", err)
			cancelIteration()
			continue
		}

		receiptsCh, err := s.genReceipts(ctx, commonIt)
		if err != nil {
			s.logError("Failed to get receipts", err)
			cancelIteration()
			continue
		}

		eventsCh, err := s.genEvents(ctx, commonIt)
		if err != nil {
			s.logError("Failed to get events", err)
			cancelIteration()
			continue
		}

		blocksCh := pipeline.Bridge(ctx, s.processSpecBlockParts(ctx, nextHeight, pipeline.FanIn(ctx,
			pipeline.Stage(ctx, headersAndSigsCh, specBlockPartsFunc[specBlockHeaderAndSigs]),
			pipeline.Stage(ctx, blockBodiesCh, specBlockPartsFunc[specBlockBody]),
			pipeline.Stage(ctx, txsCh, specBlockPartsFunc[specTransactions]),
			pipeline.Stage(ctx, receiptsCh, specBlockPartsFunc[specReceipts]),
			pipeline.Stage(ctx, eventsCh, specBlockPartsFunc[specEvents]),
		)))

		for b := range blocksCh {
			if b.err != nil {
				// cannot process any more blocks
				s.log.Errorw("Failed to process block", "err", b.err)
				cancelIteration()
				break
			}

			err = s.blockchain.Store(b.block, b.commitments, b.stateUpdate, b.newClasses)
			if err != nil {
				s.log.Errorw("Failed to Store Block", "number", b.block.Number, "err", err)
				cancelIteration()
				break
			} else {
				s.log.Infow("Stored Block", "number", b.block.Number, "hash", b.block.Hash.ShortString(), "root",
					b.block.GlobalStateRoot.ShortString())
			}
		}
		cancelIteration()
	}
}

func specBlockPartsFunc[T specBlockHeaderAndSigs | specBlockBody | specTransactions | specReceipts | specEvents](i T) specBlockParts {
	return specBlockParts(i)
}

func (s *syncService) logError(msg string, err error) {
	if !errors.Is(err, context.Canceled) {
		s.log.Errorw(msg, "err", err)
	}
}

// blockBody is used to mange all the different parts of the blocks require to store the block in the blockchain.Store()
type blockBody struct {
	block       *core.Block
	stateUpdate *core.StateUpdate
	newClasses  map[felt.Felt]core.Class
	commitments *core.BlockCommitments
	err         error
}

func (s *syncService) processSpecBlockParts(ctx context.Context, startingBlockNum uint64, specBlockPartsCh <-chan specBlockParts) <-chan <-chan blockBody {
	orderedBlockBodiesCh := make(chan (<-chan blockBody))

	go func() {
		defer close(orderedBlockBodiesCh)

		specBlockHeadersAndSigsM := make(map[uint64]specBlockHeaderAndSigs)
		specBlockBodiesM := make(map[uint64]specBlockBody)
		specTransactionsM := make(map[uint64]specTransactions)
		specReceiptsM := make(map[uint64]specReceipts)
		specEventsM := make(map[uint64]specEvents)

		curBlockNum := startingBlockNum
		for part := range specBlockPartsCh {
			select {
			case <-ctx.Done():
			default:
				switch p := part.(type) {
				case specBlockHeaderAndSigs:
					fmt.Printf("Received Block Header and Signatures for block number: %v\n", p.header.Number)
					if _, ok := specBlockHeadersAndSigsM[part.blockNumber()]; !ok {
						specBlockHeadersAndSigsM[part.blockNumber()] = p
					}
				case specBlockBody:
					fmt.Printf("Received Block Body parts            for block number: %v\n", p.id.Number)
					if _, ok := specBlockBodiesM[part.blockNumber()]; !ok {
						specBlockBodiesM[part.blockNumber()] = p
					}
				case specTransactions:
					fmt.Printf("Received Transactions                for block number: %v\n", p.id.Number)
					if _, ok := specTransactionsM[part.blockNumber()]; !ok {
						specTransactionsM[part.blockNumber()] = p
					}
				case specReceipts:
					fmt.Printf("Received Receipts                    for block number: %v\n", p.id.Number)
					if _, ok := specReceiptsM[part.blockNumber()]; !ok {
						specReceiptsM[part.blockNumber()] = p
					}
				case specEvents:
					fmt.Printf("Received Events                      for block number: %v\n", p.id.Number)
					if _, ok := specEventsM[part.blockNumber()]; !ok {
						specEventsM[part.blockNumber()] = p
					}
				}

				headerAndSig, ok1 := specBlockHeadersAndSigsM[curBlockNum]
				body, ok2 := specBlockBodiesM[curBlockNum]
				txs, ok3 := specTransactionsM[curBlockNum]
				rs, ok4 := specReceiptsM[curBlockNum]
				es, ok5 := specEventsM[curBlockNum]
				if ok1 && ok2 && ok3 && ok4 && ok5 {
					fmt.Println("----- Received all block parts from peers for block number:", curBlockNum, "-----")

					select {
					case <-ctx.Done():
					default:
						prevBlockRoot := &felt.Zero
						if curBlockNum > 0 {
							// First check cache if the header is not present, then get it from the db.
							if oldHeader, ok := specBlockHeadersAndSigsM[curBlockNum-1]; ok {
								prevBlockRoot = p2p2core.AdaptHash(oldHeader.header.State.Root)
							} else {
								oldHeader, err := s.blockchain.BlockHeaderByNumber(curBlockNum - 1)
								if err != nil {
									s.log.Errorw("Failed to get Header", "number", curBlockNum, "err", err)
									return
								}
								prevBlockRoot = oldHeader.GlobalStateRoot
							}
						}

						orderedBlockBodiesCh <- s.adaptAndSanityCheckBlock(ctx, headerAndSig.header, headerAndSig.sig, body.stateDiff,
							body.classes, body.proof, txs.txs, rs.receipts, es.events, prevBlockRoot)
					}

					if curBlockNum > 0 {
						delete(specBlockHeadersAndSigsM, curBlockNum-1)
					}
					delete(specBlockBodiesM, curBlockNum)
					delete(specTransactionsM, curBlockNum)
					delete(specReceiptsM, curBlockNum)
					delete(specEventsM, curBlockNum)
					curBlockNum++
				}
			}
		}
	}()
	return orderedBlockBodiesCh
}

func (s *syncService) adaptAndSanityCheckBlock(ctx context.Context, header *spec.BlockHeader, sig *spec.Signatures, diff *spec.StateDiff,
	classes *spec.Classes, proof *spec.BlockProof, txs *spec.Transactions, receipts *spec.Receipts, events *spec.Events,
	prevBlockRoot *felt.Felt,
) <-chan blockBody {
	bodyCh := make(chan blockBody)
	go func() {
		defer close(bodyCh)
		select {
		case <-ctx.Done():
			bodyCh <- blockBody{err: ctx.Err()}
		default:
			coreBlock := new(core.Block)

			var coreTxs []core.Transaction
			for _, i := range txs.GetItems() {
				coreTxs = append(coreTxs, p2p2core.AdaptTransaction(i, s.network))
			}

			coreBlock.Transactions = coreTxs

			txHashEventsM := make(map[felt.Felt][]*core.Event)
			for _, item := range events.GetItems() {
				txH := p2p2core.AdaptHash(item.TransactionHash)
				txHashEventsM[*txH] = append(txHashEventsM[*txH], p2p2core.AdaptEvent(item))
			}

			coreReceipts := utils.Map(receipts.GetItems(), p2p2core.AdaptReceipt)
			coreReceipts = utils.Map(coreReceipts, func(r *core.TransactionReceipt) *core.TransactionReceipt {
				r.Events = txHashEventsM[*r.TransactionHash]
				return r
			})
			coreBlock.Receipts = coreReceipts

			var err error

			coreHeader := p2p2core.AdaptBlockHeader(header)
			coreHeader.Signatures = utils.Map(sig.GetSignatures(), p2p2core.AdaptSignature)

			coreBlock.Header = &coreHeader
			coreBlock.EventsBloom = core.EventsBloom(coreBlock.Receipts)
			coreBlock.Hash, err = core.BlockHash(coreBlock)
			if err != nil {
				bodyCh <- blockBody{err: fmt.Errorf("block hash calculation error: %v", err)}
				return
			}

			newClasses := make(map[felt.Felt]core.Class)
			for _, i := range classes.GetClasses() {
				coreC := p2p2core.AdaptClass(i)
				h, err := coreC.Hash()
				if err != nil {
					bodyCh <- blockBody{err: fmt.Errorf("class hash calculation error: %v", err)}
					return
				}
				newClasses[*h] = coreC
			}

			// Build State update
			// Note: Parts of the State Update are created from Blockchain object as the Store and SanityCheck functions require a State
			// Update but there is no such message in P2P.

			stateUpdate := &core.StateUpdate{
				BlockHash: coreBlock.Hash,
				NewRoot:   coreBlock.GlobalStateRoot,
				OldRoot:   prevBlockRoot,
				StateDiff: p2p2core.AdaptStateDiff(diff, classes.GetClasses()),
			}

			commitments, err := s.blockchain.SanityCheckNewHeight(coreBlock, stateUpdate, newClasses)
			if err != nil {
				bodyCh <- blockBody{err: fmt.Errorf("sanity check error: %v for block number: %v", err, coreBlock.Number)}
				return
			}

			select {
			case <-ctx.Done():
			case bodyCh <- blockBody{block: coreBlock, stateUpdate: stateUpdate, newClasses: newClasses, commitments: commitments}:
			}
		}
	}()

	return bodyCh
}

type specBlockParts interface {
	blockNumber() uint64
}

type specBlockHeaderAndSigs struct {
	header *spec.BlockHeader
	sig    *spec.Signatures
}

func (s specBlockHeaderAndSigs) blockNumber() uint64 {
	return s.header.Number
}

func (s *syncService) genHeadersAndSigs(ctx context.Context, it *spec.Iteration) (<-chan specBlockHeaderAndSigs, error) {
	headersIt, err := s.client.RequestBlockHeaders(ctx, &spec.BlockHeadersRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	headersAndSigCh := make(chan specBlockHeaderAndSigs)
	go func() {
		defer close(headersAndSigCh)

	iteratorLoop:
		for res, valid := headersIt(); valid; res, valid = headersIt() {
			headerAndSig := specBlockHeaderAndSigs{}
			for _, part := range res.GetPart() {
				switch part.HeaderMessage.(type) {
				case *spec.BlockHeadersResponsePart_Header:
					headerAndSig.header = part.GetHeader()
				case *spec.BlockHeadersResponsePart_Signatures:
					headerAndSig.sig = part.GetSignatures()
				case *spec.BlockHeadersResponsePart_Fin:
					// received all the parts of BlockHeadersResponse
					// To close the stream we need to do an extra read from the stream which will cause and error thus closing the stream
					continue iteratorLoop
				}
			}

			select {
			case <-ctx.Done():
				// Consume everything from the stream which will eventually break the for iteration loop.
				// Todo: modify the usage of stream, so that we use libp2p network stream which gives the ability to close it.
				continue
			case headersAndSigCh <- headerAndSig:
			}
		}
	}()

	return headersAndSigCh, nil
}

type specBlockBody struct {
	id        *spec.BlockID
	proof     *spec.BlockProof
	classes   *spec.Classes
	stateDiff *spec.StateDiff
}

func (s specBlockBody) blockNumber() uint64 {
	return s.id.Number
}

func (s *syncService) genBlockBodies(ctx context.Context, it *spec.Iteration) (<-chan specBlockBody, error) {
	blockIt, err := s.client.RequestBlockBodies(ctx, &spec.BlockBodiesRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	specBodiesCh := make(chan specBlockBody)
	go func() {
		defer close(specBodiesCh)
		curBlockBody := new(specBlockBody)
		// Assumes that all parts of the same block will arrive before the next block parts
		// Todo: the above assumption may not be true. A peer may decide to send different parts of the block in different order
		// If the above assumption is not true we should return separate channels for each of the parts. Also, see todo above specBlockBody
		// on line 317 in p2p/sync.go
		for res, valid := blockIt(); valid; res, valid = blockIt() {
			switch res.BodyMessage.(type) {
			case *spec.BlockBodiesResponse_Classes:
				if curBlockBody.id == nil {
					curBlockBody.id = res.GetId()
				}
				curBlockBody.classes = res.GetClasses()
			case *spec.BlockBodiesResponse_Diff:
				if curBlockBody.id == nil {
					curBlockBody.id = res.GetId()
				}
				curBlockBody.stateDiff = res.GetDiff()
			case *spec.BlockBodiesResponse_Proof:
				if curBlockBody.id == nil {
					curBlockBody.id = res.GetId()
				}
				curBlockBody.proof = res.GetProof()
			case *spec.BlockBodiesResponse_Fin:
				if curBlockBody.id != nil {
					select {
					case <-ctx.Done():
					default:
						specBodiesCh <- *curBlockBody
						curBlockBody = new(specBlockBody)
					}
				}
			}
		}
	}()

	return specBodiesCh, nil
}

type specReceipts struct {
	id       *spec.BlockID
	receipts *spec.Receipts
}

func (s specReceipts) blockNumber() uint64 {
	return s.id.Number
}

func (s *syncService) genReceipts(ctx context.Context, it *spec.Iteration) (<-chan specReceipts, error) {
	receiptsIt, err := s.client.RequestReceipts(ctx, &spec.ReceiptsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	receiptsCh := make(chan specReceipts)
	go func() {
		defer close(receiptsCh)

		for res, valid := receiptsIt(); valid; res, valid = receiptsIt() {
			switch res.Responses.(type) {
			case *spec.ReceiptsResponse_Receipts:
				select {
				case <-ctx.Done():
				case receiptsCh <- specReceipts{res.GetId(), res.GetReceipts()}:
				}
			case *spec.ReceiptsResponse_Fin:
				continue
			}
		}
	}()

	return receiptsCh, nil
}

type specEvents struct {
	id     *spec.BlockID
	events *spec.Events
}

func (s specEvents) blockNumber() uint64 {
	return s.id.Number
}

func (s *syncService) genEvents(ctx context.Context, it *spec.Iteration) (<-chan specEvents, error) {
	eventsIt, err := s.client.RequestEvents(ctx, &spec.EventsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	eventsCh := make(chan specEvents)
	go func() {
		defer close(eventsCh)
		for res, valid := eventsIt(); valid; res, valid = eventsIt() {
			switch res.Responses.(type) {
			case *spec.EventsResponse_Events:
				select {
				case <-ctx.Done():
				case eventsCh <- specEvents{res.GetId(), res.GetEvents()}:
				}
			case *spec.EventsResponse_Fin:
				continue
			}
		}
	}()
	return eventsCh, nil
}

type specTransactions struct {
	id  *spec.BlockID
	txs *spec.Transactions
}

func (s specTransactions) blockNumber() uint64 {
	return s.id.Number
}

func (s *syncService) genTransactions(ctx context.Context, it *spec.Iteration) (<-chan specTransactions, error) {
	txsIt, err := s.client.RequestTransactions(ctx, &spec.TransactionsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	txsCh := make(chan specTransactions)
	go func() {
		defer close(txsCh)
		for res, valid := txsIt(); valid; res, valid = txsIt() {
			switch res.Responses.(type) {
			case *spec.TransactionsResponse_Transactions:
				select {
				case <-ctx.Done():
				case txsCh <- specTransactions{res.GetId(), res.GetTransactions()}:
				}
			case *spec.TransactionsResponse_Fin:
				continue
			}
		}
	}()
	return txsCh, nil
}

func (s *syncService) randomPeer() peer.ID {
	peers := s.host.Peerstore().Peers()

	// todo do not request same block from all peers
	peers = utils.Filter(peers, func(peerID peer.ID) bool {
		return peerID != s.host.ID()
	})
	if len(peers) == 0 {
		panic("No peers available")
	}

	peer := peers[rand.Intn(len(peers))]

	fmt.Println("Number of peers", len(peers))
	fmt.Println("Random chosen peer's Info", s.host.Peerstore().PeerInfo(peer))

	return peer
}

func (s *syncService) randomPeerStream(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
	return s.host.NewStream(ctx, s.randomPeer(), pids...)
}

func (s *syncService) createIterator(start, limit uint64) *spec.Iteration {
	if limit == 0 {
		limit = 1
	}
	return &spec.Iteration{
		Start:     &spec.Iteration_BlockNumber{start},
		Direction: spec.Iteration_Forward,
		Limit:     limit,
		Step:      1,
	}
}

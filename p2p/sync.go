package p2p

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"time"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/p2p/starknet"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	junoSync "github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/utils/pipeline"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/zap"
)

type syncService struct {
	host    host.Host
	network *utils.Network
	client  *starknet.Client // todo: merge all the functionality of Client with p2p SyncService

	blockchain *blockchain.Blockchain
	listener   junoSync.EventListener
	log        utils.SimpleLogger
}

func newSyncService(bc *blockchain.Blockchain, h host.Host, n *utils.Network, log utils.SimpleLogger) *syncService {
	return &syncService{
		host:       h,
		network:    n,
		blockchain: bc,
		log:        log,
		listener:   &junoSync.SelectiveListener{},
	}
}

func (s *syncService) start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.client = starknet.NewClient(s.randomPeerStream, s.network, s.log)

	for i := 0; ; i++ {
		if err := ctx.Err(); err != nil {
			break
		}
		s.log.Debugw("Continuous iteration", "i", i)

		iterCtx, cancelIteration := context.WithCancel(ctx)
		nextHeight, err := s.getNextHeight()
		if err != nil {
			s.logError("Failed to get current height", err)
			cancelIteration()
			continue
		}

		s.log.Infow("Start Pipeline", "Current height", nextHeight-1, "Start", nextHeight)

		// todo change iteration to fetch several objects uint64(min(blockBehind, maxBlocks))
		blockNumber := uint64(nextHeight)
		if err := s.processBlock(iterCtx, blockNumber); err != nil {
			s.logError("Failed to process block", fmt.Errorf("blockNumber: %d, err: %w", blockNumber, err))
			cancelIteration()
			continue
		}

		cancelIteration()
	}
}

func (s *syncService) getNextHeight() (int, error) {
	curHeight, err := s.blockchain.Height()
	if err == nil {
		return int(curHeight) + 1, nil
	} else if errors.Is(err, db.ErrKeyNotFound) {
		return 0, nil
	}
	return 0, err
}

func (s *syncService) processBlock(ctx context.Context, blockNumber uint64) error {
	headersAndSigsCh, err := s.genHeadersAndSigs(ctx, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get block headers parts: %w", err)
	}

	txsCh, err := s.genTransactions(ctx, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get transactions: %w", err)
	}

	eventsCh, err := s.genEvents(ctx, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	classesCh, err := s.genClasses(ctx, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get classes: %w", err)
	}

	stateDiffsCh, err := s.genStateDiffs(ctx, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get state diffs: %w", err)
	}

	blocksCh := pipeline.Bridge(ctx, s.processSpecBlockParts(ctx, blockNumber, pipeline.FanIn(ctx,
		pipeline.Stage(ctx, headersAndSigsCh, specBlockPartsFunc[specBlockHeaderAndSigs]),
		pipeline.Stage(ctx, classesCh, specBlockPartsFunc[specClasses]),
		pipeline.Stage(ctx, stateDiffsCh, specBlockPartsFunc[specContractDiffs]),
		pipeline.Stage(ctx, txsCh, specBlockPartsFunc[specTxWithReceipts]),
		pipeline.Stage(ctx, eventsCh, specBlockPartsFunc[specEvents]),
	)))

	for b := range blocksCh {
		if b.err != nil {
			return fmt.Errorf("failed to process block: %w", b.err)
		}

		storeTimer := time.Now()
		if err := s.blockchain.Store(b.block, b.commitments, b.stateUpdate, b.newClasses); err != nil {
			return fmt.Errorf("failed to store block: %w", err)
		}

		s.log.Infow("Stored Block", "number", b.block.Number, "hash", b.block.Hash.ShortString(),
			"root", b.block.GlobalStateRoot.ShortString())
		s.listener.OnSyncStepDone(junoSync.OpStore, b.block.Number, time.Since(storeTimer))
	}
	return nil
}

func specBlockPartsFunc[T specBlockHeaderAndSigs | specTxWithReceipts | specEvents | specClasses | specContractDiffs](i T) specBlockParts {
	return specBlockParts(i)
}

func (s *syncService) logError(msg string, err error) {
	if !errors.Is(err, context.Canceled) {
		var log utils.SimpleLogger
		if v, ok := s.log.(*utils.ZapLogger); ok {
			enhancedLogger := v.SugaredLogger.Desugar().WithOptions(zap.AddCallerSkip(1)).Sugar()
			log = &utils.ZapLogger{SugaredLogger: enhancedLogger}
		} else {
			log = s.log
		}

		log.Errorw(msg, "err", err)
	} else {
		s.log.Debugw("Sync context canceled")
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

//nolint:gocyclo
func (s *syncService) processSpecBlockParts(
	ctx context.Context, startingBlockNum uint64, specBlockPartsCh <-chan specBlockParts,
) <-chan <-chan blockBody {
	orderedBlockBodiesCh := make(chan (<-chan blockBody))

	go func() {
		defer close(orderedBlockBodiesCh)

		specBlockHeadersAndSigsM := make(map[uint64]specBlockHeaderAndSigs)
		specClassesM := make(map[uint64]specClasses)
		specTransactionsM := make(map[uint64]specTxWithReceipts)
		specEventsM := make(map[uint64]specEvents)
		specContractDiffsM := make(map[uint64]specContractDiffs)

		curBlockNum := startingBlockNum
		for part := range specBlockPartsCh {
			select {
			case <-ctx.Done():
			default:
				switch p := part.(type) {
				case specBlockHeaderAndSigs:
					s.log.Debugw("Received Block Header with signatures", "blockNumber", p.blockNumber())
					if _, ok := specBlockHeadersAndSigsM[part.blockNumber()]; !ok {
						specBlockHeadersAndSigsM[part.blockNumber()] = p
					}
				case specTxWithReceipts:
					s.log.Debugw("Received Transactions with receipts", "blockNumber", p.blockNumber(), "txLen", len(p.txs))
					if _, ok := specTransactionsM[part.blockNumber()]; !ok {
						specTransactionsM[part.blockNumber()] = p
					}
				case specEvents:
					s.log.Debugw("Received Events", "blockNumber", p.blockNumber(), "len", len(p.events))
					if _, ok := specEventsM[part.blockNumber()]; !ok {
						specEventsM[part.blockNumber()] = p
					}
				case specClasses:
					s.log.Debugw("Received Classes", "blockNumber", p.blockNumber())
					if _, ok := specClassesM[part.blockNumber()]; !ok {
						specClassesM[part.blockNumber()] = p
					}
				case specContractDiffs:
					s.log.Debugw("Received ContractDiffs", "blockNumber", p.blockNumber())
					if _, ok := specContractDiffsM[part.blockNumber()]; !ok {
						specContractDiffsM[part.blockNumber()] = p
					}
				default:
					s.log.Warnw("Unsupported part type", "blockNumber", part.blockNumber(), "type", reflect.TypeOf(p))
				}

				headerAndSig, okHeader := specBlockHeadersAndSigsM[curBlockNum]
				txs, okTxs := specTransactionsM[curBlockNum]
				es, okEvents := specEventsM[curBlockNum]
				cls, okClasses := specClassesM[curBlockNum]
				diffs, okDiffs := specContractDiffsM[curBlockNum]
				if okHeader && okTxs && okEvents && okClasses && okDiffs {
					s.log.Debugw(fmt.Sprintf("----- Received all block parts from peers for block number %d-----", curBlockNum))

					select {
					case <-ctx.Done():
					default:
						prevBlockRoot := &felt.Zero
						if curBlockNum > 0 {
							// First check cache if the header is not present, then get it from the db.
							if oldHeader, ok := specBlockHeadersAndSigsM[curBlockNum-1]; ok {
								prevBlockRoot = p2p2core.AdaptHash(oldHeader.header.StateRoot)
							} else {
								oldHeader, err := s.blockchain.BlockHeaderByNumber(curBlockNum - 1)
								if err != nil {
									s.log.Errorw("Failed to get Header", "number", curBlockNum, "err", err)
									return
								}
								prevBlockRoot = oldHeader.GlobalStateRoot
							}
						}

						orderedBlockBodiesCh <- s.adaptAndSanityCheckBlock(ctx, headerAndSig.header, diffs.contractDiffs,
							cls.classes, txs.txs, txs.receipts, es.events, prevBlockRoot)
					}

					if curBlockNum > 0 {
						delete(specBlockHeadersAndSigsM, curBlockNum-1)
					}
					delete(specTransactionsM, curBlockNum)
					delete(specEventsM, curBlockNum)
					curBlockNum++
				}
			}
		}
	}()
	return orderedBlockBodiesCh
}

//nolint:gocyclo
func (s *syncService) adaptAndSanityCheckBlock(ctx context.Context, header *spec.SignedBlockHeader, contractDiffs []*spec.ContractDiff,
	classes []*spec.Class, txs []*spec.Transaction, receipts []*spec.Receipt, events []*spec.Event, prevBlockRoot *felt.Felt,
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
			for _, tx := range txs {
				coreTxs = append(coreTxs, p2p2core.AdaptTransaction(tx, s.network))
			}
			coreBlock.Transactions = coreTxs

			txHashEventsM := make(map[felt.Felt][]*core.Event)
			for _, event := range events {
				txH := p2p2core.AdaptHash(event.TransactionHash)
				txHashEventsM[*txH] = append(txHashEventsM[*txH], p2p2core.AdaptEvent(event))
			}

			coreReceipts := make([]*core.TransactionReceipt, 0, len(receipts))
			for i, r := range receipts {
				coreReceipt := p2p2core.AdaptReceipt(r, coreTxs[i].Hash())
				coreReceipt.Events = txHashEventsM[*coreReceipt.TransactionHash]
				coreReceipts = append(coreReceipts, coreReceipt)
			}
			coreBlock.Receipts = coreReceipts

			eventsBloom := core.EventsBloom(coreBlock.Receipts)
			coreBlock.Header = p2p2core.AdaptBlockHeader(header, eventsBloom)

			if int(coreBlock.TransactionCount) != len(coreBlock.Transactions) {
				s.log.Errorw(
					"Number of transactions != count",
					"transactionCount",
					coreBlock.TransactionCount,
					"len(transactions)",
					len(coreBlock.Transactions),
				)
				return
			}
			if int(coreBlock.EventCount) != len(events) {
				s.log.Errorw(
					"Number of events != count",
					"eventCount",
					coreBlock.EventCount,
					"len(events)",
					len(events),
				)
				return
			}

			newClasses := make(map[felt.Felt]core.Class)
			for _, cls := range classes {
				coreC := p2p2core.AdaptClass(cls)
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

			stateReader, stateCloser, err := s.blockchain.StateAtBlockNumber(coreBlock.Number - 1)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				// todo(kirill) change to shutdown
				panic(err)
			}
			defer func() {
				if stateCloser == nil {
					return
				}

				if closeErr := stateCloser(); closeErr != nil {
					s.log.Errorw("Failed to close state reader", "err", closeErr)
				}
			}()

			stateUpdate := &core.StateUpdate{
				BlockHash: coreBlock.Hash,
				NewRoot:   coreBlock.GlobalStateRoot,
				OldRoot:   prevBlockRoot,
				StateDiff: p2p2core.AdaptStateDiff(stateReader, contractDiffs, classes),
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
	header *spec.SignedBlockHeader
}

func (s specBlockHeaderAndSigs) blockNumber() uint64 {
	return s.header.Number
}

func (s *syncService) genHeadersAndSigs(ctx context.Context, blockNumber uint64) (<-chan specBlockHeaderAndSigs, error) {
	it := s.createIteratorForBlock(blockNumber)
	headersIt, err := s.client.RequestBlockHeaders(ctx, &spec.BlockHeadersRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	headersAndSigCh := make(chan specBlockHeaderAndSigs)
	go func() {
		defer close(headersAndSigCh)

	loop:
		for res := range headersIt {
			headerAndSig := specBlockHeaderAndSigs{}
			switch v := res.HeaderMessage.(type) {
			case *spec.BlockHeadersResponse_Header:
				headerAndSig.header = v.Header
			case *spec.BlockHeadersResponse_Fin:
				break loop
			default:
				s.log.Warnw("Unexpected HeaderMessage from getBlockHeaders", "v", v)
				break loop
			}

			select {
			case <-ctx.Done():
				break
			case headersAndSigCh <- headerAndSig:
			}
		}
	}()

	return headersAndSigCh, nil
}

type specClasses struct {
	number  uint64
	classes []*spec.Class
}

func (s specClasses) blockNumber() uint64 {
	return s.number
}

func (s *syncService) genClasses(ctx context.Context, blockNumber uint64) (<-chan specClasses, error) {
	it := s.createIteratorForBlock(blockNumber)
	classesIt, err := s.client.RequestClasses(ctx, &spec.ClassesRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	classesCh := make(chan specClasses)
	go func() {
		defer close(classesCh)

		var classes []*spec.Class
	loop:
		for res := range classesIt {
			switch v := res.ClassMessage.(type) {
			case *spec.ClassesResponse_Class:
				classes = append(classes, v.Class)
			case *spec.ClassesResponse_Fin:
				break loop
			default:
				s.log.Warnw("Unexpected ClassMessage from getClasses", "v", v)
				break loop
			}
		}

		select {
		case <-ctx.Done():
		case classesCh <- specClasses{
			number:  blockNumber,
			classes: classes,
		}:
			s.log.Debugw("Received classes for block", "blockNumber", blockNumber, "lenClasses", len(classes))
		}
	}()
	return classesCh, nil
}

type specContractDiffs struct {
	number        uint64
	contractDiffs []*spec.ContractDiff
}

func (s specContractDiffs) blockNumber() uint64 {
	return s.number
}

func (s *syncService) genStateDiffs(ctx context.Context, blockNumber uint64) (<-chan specContractDiffs, error) {
	it := s.createIteratorForBlock(blockNumber)
	stateDiffsIt, err := s.client.RequestStateDiffs(ctx, &spec.StateDiffsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	stateDiffsCh := make(chan specContractDiffs)
	go func() {
		defer close(stateDiffsCh)

		var contractDiffs []*spec.ContractDiff

	loop:
		for res := range stateDiffsIt {
			switch v := res.StateDiffMessage.(type) {
			case *spec.StateDiffsResponse_ContractDiff:
				contractDiffs = append(contractDiffs, v.ContractDiff)
			case *spec.StateDiffsResponse_DeclaredClass:
				s.log.Warnw("Unimplemented message StateDiffsResponse_DeclaredClass")
			case *spec.StateDiffsResponse_Fin:
				break loop
			default:
				s.log.Warnw("Unexpected ClassMessage from getStateDiffs", "v", v)
				break loop
			}
		}

		select {
		case <-ctx.Done():
		case stateDiffsCh <- specContractDiffs{
			number:        blockNumber,
			contractDiffs: contractDiffs,
		}:
		}
	}()
	return stateDiffsCh, nil
}

type specEvents struct {
	number uint64
	events []*spec.Event
}

func (s specEvents) blockNumber() uint64 {
	return s.number
}

func (s *syncService) genEvents(ctx context.Context, blockNumber uint64) (<-chan specEvents, error) {
	it := s.createIteratorForBlock(blockNumber)
	eventsIt, err := s.client.RequestEvents(ctx, &spec.EventsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	eventsCh := make(chan specEvents)
	go func() {
		defer close(eventsCh)

		var events []*spec.Event

	loop:
		for res := range eventsIt {
			switch v := res.EventMessage.(type) {
			case *spec.EventsResponse_Event:
				events = append(events, v.Event)
			case *spec.EventsResponse_Fin:
				break loop
			default:
				s.log.Warnw("Unexpected EventMessage from getEvents", "v", v)
				break loop
			}
		}

		select {
		case <-ctx.Done():
		case eventsCh <- specEvents{
			number: blockNumber,
			events: events,
		}:
		}
	}()
	return eventsCh, nil
}

type specTxWithReceipts struct {
	number   uint64
	txs      []*spec.Transaction
	receipts []*spec.Receipt
}

func (s specTxWithReceipts) blockNumber() uint64 {
	return s.number
}

func (s *syncService) genTransactions(ctx context.Context, blockNumber uint64) (<-chan specTxWithReceipts, error) {
	it := s.createIteratorForBlock(blockNumber)
	txsIt, err := s.client.RequestTransactions(ctx, &spec.TransactionsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	txsCh := make(chan specTxWithReceipts)
	go func() {
		defer close(txsCh)

		var (
			transactions []*spec.Transaction
			receipts     []*spec.Receipt
		)

	loop:
		for res := range txsIt {
			switch v := res.TransactionMessage.(type) {
			case *spec.TransactionsResponse_TransactionWithReceipt:
				txWithReceipt := v.TransactionWithReceipt
				transactions = append(transactions, txWithReceipt.Transaction)
				receipts = append(receipts, txWithReceipt.Receipt)
			case *spec.TransactionsResponse_Fin:
				break loop
			default:
				s.log.Warnw("Unexpected TransactionMessage from getTransactions", "v", v)
				break loop
			}
		}

		s.log.Debugw("Transactions length", "len", len(transactions))
		spexTxs := specTxWithReceipts{
			number:   blockNumber,
			txs:      transactions,
			receipts: receipts,
		}
		select {
		case <-ctx.Done():
			return
		case txsCh <- spexTxs:
		}
	}()
	return txsCh, nil
}

func (s *syncService) randomPeer() peer.ID {
	store := s.host.Peerstore()
	// todo do not request same block from all peers
	peers := utils.Filter(store.Peers(), func(peerID peer.ID) bool {
		return peerID != s.host.ID()
	})
	if len(peers) == 0 {
		return ""
	}

	p := peers[rand.Intn(len(peers))] //nolint:gosec

	s.log.Debugw("Number of peers", "len", len(peers))
	s.log.Debugw("Random chosen peer's info", "peerInfo", store.PeerInfo(p))

	return p
}

var errNoPeers = errors.New("no peers available")

func (s *syncService) randomPeerStream(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
	randPeer := s.randomPeer()
	if randPeer == "" {
		return nil, errNoPeers
	}
	stream, err := s.host.NewStream(ctx, randPeer, pids...)
	if err != nil {
		s.log.Debugw("Error creating stream", "peer", randPeer, "err", err)
		s.removePeer(randPeer)
		return nil, err
	}
	return stream, err
}

func (s *syncService) removePeer(id peer.ID) {
	s.log.Debugw("Removing peer", "peerID", id)
	s.host.Peerstore().RemovePeer(id)
	s.host.Peerstore().ClearAddrs(id)
}

func (s *syncService) createIteratorForBlock(blockNumber uint64) *spec.Iteration {
	return &spec.Iteration{
		Start:     &spec.Iteration_BlockNumber{BlockNumber: blockNumber},
		Direction: spec.Iteration_Forward,
		Limit:     1,
		Step:      1,
	}
}

func (s *syncService) WithListener(l junoSync.EventListener) {
	s.listener = l
}

//nolint:unused
func (s *syncService) sleep(d time.Duration) {
	s.log.Debugw("Sleeping...", "for", d)
	time.Sleep(d)
}

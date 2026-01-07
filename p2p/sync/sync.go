package sync

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/p2p/hashstorage"
	junoSync "github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/utils/pipeline"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/class"
	syncclass "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/class"
	synccommon "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/event"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/header"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/receipt"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/state"
	synctransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/transaction"
	"go.uber.org/zap"
)

type Service struct {
	blockchain *blockchain.Blockchain
	log        utils.SimpleLogger

	blockFetcher *BlockFetcher
	blockCh      chan BlockBody
}

func New(
	bc *blockchain.Blockchain,
	log utils.SimpleLogger,
	blockFetcher *BlockFetcher,
) *Service {
	return &Service{
		blockchain:   bc,
		log:          log,
		blockFetcher: blockFetcher,
		blockCh:      make(chan BlockBody),
	}
}

func (s *Service) Listen() <-chan BlockBody {
	return s.blockCh
}

func (s *Service) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(s.blockCh)

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
		if err = s.blockFetcher.ProcessBlock(iterCtx, blockNumber, s.blockCh); err != nil {
			s.logError("Failed to process block", fmt.Errorf("blockNumber: %d, err: %w", blockNumber, err))
			cancelIteration()
			continue
		}
		cancelIteration()
	}
}

func (s *Service) WithListener(l junoSync.EventListener) {
	s.blockFetcher.WithListener(l)
}

func (s *Service) getNextHeight() (int, error) {
	curHeight, err := s.blockchain.Height()
	if err == nil {
		return int(curHeight) + 1, nil
	} else if errors.Is(err, db.ErrKeyNotFound) {
		return 0, nil
	}
	return 0, err
}

func (s *Service) logError(msg string, err error) {
	if !errors.Is(err, context.Canceled) {
		var log utils.SimpleLogger
		if v, ok := s.log.(*utils.ZapLogger); ok {
			log = v.WithOptions(zap.AddCallerSkip(1))
		} else {
			log = s.log
		}

		log.Errorw(msg, "err", err)
	} else {
		s.log.Debugw("Sync context canceled")
	}
}

type BlockFetcher struct {
	network    *utils.Network
	client     *Client // todo: merge all the functionality of Client with p2p SyncService
	blockchain *blockchain.Blockchain
	listener   junoSync.EventListener
	log        utils.SimpleLogger
}

func NewBlockFetcher(
	bc *blockchain.Blockchain,
	h host.Host,
	n *utils.Network,
	log utils.SimpleLogger,
) BlockFetcher {
	return BlockFetcher{
		network:    n,
		blockchain: bc,
		log:        log,
		listener:   &junoSync.SelectiveListener{},
		client:     NewClient(randomPeerStream(h, log), n, log),
	}
}

func (s *BlockFetcher) ProcessBlock(
	ctx context.Context,
	blockNumber uint64,
	outputs chan<- BlockBody,
) error {
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

	pipeline.Bridge(
		ctx,
		outputs,
		s.processSpecBlockParts(
			ctx,
			blockNumber,
			pipeline.FanIn(ctx,
				pipeline.Stage(ctx, headersAndSigsCh, specBlockPartsFunc[specBlockHeaderAndSigs]),
				pipeline.Stage(ctx, classesCh, specBlockPartsFunc[specClasses]),
				pipeline.Stage(ctx, stateDiffsCh, specBlockPartsFunc[specContractDiffs]),
				pipeline.Stage(ctx, txsCh, specBlockPartsFunc[specTxWithReceipts]),
				pipeline.Stage(ctx, eventsCh, specBlockPartsFunc[specEvents]),
			),
		),
	)

	return nil
}

func specBlockPartsFunc[T specBlockHeaderAndSigs | specTxWithReceipts | specEvents | specClasses | specContractDiffs](i T) specBlockParts {
	return specBlockParts(i)
}

// BlockBody is used to mange all the different parts of the blocks require to store the block in the blockchain.Store()
type BlockBody struct {
	Block       *core.Block
	StateUpdate *core.StateUpdate
	NewClasses  map[felt.Felt]core.ClassDefinition
	Commitments *core.BlockCommitments
	Err         error
}

//nolint:gocyclo
func (s *BlockFetcher) processSpecBlockParts(
	ctx context.Context, startingBlockNum uint64, specBlockPartsCh <-chan specBlockParts,
) <-chan <-chan BlockBody {
	orderedBlockBodiesCh := make(chan (<-chan BlockBody))

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

//nolint:gocyclo,funlen
func (s *BlockFetcher) adaptAndSanityCheckBlock(
	ctx context.Context,
	header *header.SignedBlockHeader,
	contractDiffs []*state.ContractDiff,
	classes []*class.Class,
	txs []*synctransaction.TransactionInBlock,
	receipts []*receipt.Receipt,
	events []*event.Event,
	prevBlockRoot *felt.Felt,
) <-chan BlockBody {
	bodyCh := make(chan BlockBody)
	go func() {
		defer close(bodyCh)
		select {
		case <-ctx.Done():
			bodyCh <- BlockBody{Err: ctx.Err()}
		default:
			coreBlock := new(core.Block)

			coreTxs := make([]core.Transaction, len(txs))
			for i, tx := range txs {
				coreTx, err := p2p2core.AdaptTransaction(tx, s.network)
				if err != nil {
					bodyCh <- BlockBody{Err: fmt.Errorf("failed to adapt transaction: %w", err)}
					return
				}
				coreTxs[i] = coreTx
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
			header, err := p2p2core.AdaptBlockHeader(header, eventsBloom)
			if err != nil {
				bodyCh <- BlockBody{Err: fmt.Errorf("failed to adapt block header: %w", err)}
				return
			}
			coreBlock.Header = header

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

			newClasses := make(map[felt.Felt]core.ClassDefinition)
			for _, cls := range classes {
				coreC, err := p2p2core.AdaptClass(cls)
				if err != nil {
					bodyCh <- BlockBody{Err: fmt.Errorf("failed to adapt class: %w", err)}
					return
				}

				h, err := coreC.Hash()
				if err != nil {
					bodyCh <- BlockBody{Err: fmt.Errorf("class hash calculation error: %w", err)}
					return
				}
				newClasses[h] = coreC
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

			stateDiff, err := p2p2core.AdaptStateDiff(stateReader, contractDiffs, classes)
			if err != nil {
				bodyCh <- BlockBody{Err: fmt.Errorf("failed to adapt state diff: %w", err)}
				return
			}

			blockVer, err := core.ParseBlockVersion(coreBlock.ProtocolVersion)
			if err != nil {
				bodyCh <- BlockBody{Err: fmt.Errorf("failed to parse block version: %w", err)}
				return
			}

			if blockVer.LessThan(core.Ver0_13_2) && s.network.L2ChainID == "SN_SEPOLIA" {
				expectedHash := hashstorage.SepoliaBlockHashesMap[coreBlock.Number]
				post0132Hash, _, err := core.Post0132Hash(coreBlock, stateDiff)
				if err != nil {
					bodyCh <- BlockBody{Err: fmt.Errorf("failed to compute p2p hash: %w", err)}
					return
				}

				if !expectedHash.Equal(&post0132Hash) {
					err = fmt.Errorf("block hash mismatch: expected %s, got %s",
						expectedHash,
						&post0132Hash,
					)
					bodyCh <- BlockBody{Err: err}
					return
				}
			}

			stateUpdate := &core.StateUpdate{
				BlockHash: coreBlock.Hash,
				NewRoot:   coreBlock.GlobalStateRoot,
				OldRoot:   prevBlockRoot,
				StateDiff: stateDiff,
			}

			commitments, err := s.blockchain.SanityCheckNewHeight(coreBlock, stateUpdate, newClasses)
			if err != nil {
				bodyCh <- BlockBody{Err: fmt.Errorf("sanity check error: %v for block number: %v", err, coreBlock.Number)}
				return
			}

			select {
			case <-ctx.Done():
			case bodyCh <- BlockBody{Block: coreBlock, StateUpdate: stateUpdate, NewClasses: newClasses, Commitments: commitments}:
			}
		}
	}()

	return bodyCh
}

type specBlockParts interface {
	blockNumber() uint64
}

type specBlockHeaderAndSigs struct {
	header *header.SignedBlockHeader
}

func (s specBlockHeaderAndSigs) blockNumber() uint64 {
	return s.header.Number
}

func (s *BlockFetcher) genHeadersAndSigs(
	ctx context.Context,
	blockNumber uint64,
) (<-chan specBlockHeaderAndSigs, error) {
	it := s.createIteratorForBlock(blockNumber)
	headersIt, err := s.client.RequestBlockHeaders(ctx, &header.BlockHeadersRequest{Iteration: it})
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
			case *header.BlockHeadersResponse_Header:
				headerAndSig.header = v.Header
			case *header.BlockHeadersResponse_Fin:
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
	classes []*class.Class
}

func (s specClasses) blockNumber() uint64 {
	return s.number
}

func (s *BlockFetcher) genClasses(
	ctx context.Context,
	blockNumber uint64,
) (<-chan specClasses, error) {
	it := s.createIteratorForBlock(blockNumber)
	classesIt, err := s.client.RequestClasses(ctx, &syncclass.ClassesRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	classesCh := make(chan specClasses)
	go func() {
		defer close(classesCh)

		var classes []*class.Class
	loop:
		for res := range classesIt {
			switch v := res.ClassMessage.(type) {
			case *syncclass.ClassesResponse_Class:
				classes = append(classes, v.Class)
			case *syncclass.ClassesResponse_Fin:
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
	contractDiffs []*state.ContractDiff
}

func (s specContractDiffs) blockNumber() uint64 {
	return s.number
}

func (s *BlockFetcher) genStateDiffs(
	ctx context.Context,
	blockNumber uint64,
) (<-chan specContractDiffs, error) {
	it := s.createIteratorForBlock(blockNumber)
	stateDiffsIt, err := s.client.RequestStateDiffs(ctx, &state.StateDiffsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	stateDiffsCh := make(chan specContractDiffs)
	go func() {
		defer close(stateDiffsCh)

		var contractDiffs []*state.ContractDiff

	loop:
		for res := range stateDiffsIt {
			switch v := res.StateDiffMessage.(type) {
			case *state.StateDiffsResponse_ContractDiff:
				contractDiffs = append(contractDiffs, v.ContractDiff)
			case *state.StateDiffsResponse_DeclaredClass:
				s.log.Warnw("Unimplemented message StateDiffsResponse_DeclaredClass")
			case *state.StateDiffsResponse_Fin:
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
	events []*event.Event
}

func (s specEvents) blockNumber() uint64 {
	return s.number
}

func (s *BlockFetcher) genEvents(
	ctx context.Context,
	blockNumber uint64,
) (<-chan specEvents, error) {
	it := s.createIteratorForBlock(blockNumber)
	eventsIt, err := s.client.RequestEvents(ctx, &event.EventsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	eventsCh := make(chan specEvents)
	go func() {
		defer close(eventsCh)

		var events []*event.Event

	loop:
		for res := range eventsIt {
			switch v := res.EventMessage.(type) {
			case *event.EventsResponse_Event:
				events = append(events, v.Event)
			case *event.EventsResponse_Fin:
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
	txs      []*synctransaction.TransactionInBlock
	receipts []*receipt.Receipt
}

func (s specTxWithReceipts) blockNumber() uint64 {
	return s.number
}

func (s *BlockFetcher) genTransactions(
	ctx context.Context,
	blockNumber uint64,
) (<-chan specTxWithReceipts, error) {
	it := s.createIteratorForBlock(blockNumber)
	txsIt, err := s.client.RequestTransactions(ctx, &synctransaction.TransactionsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	txsCh := make(chan specTxWithReceipts)
	go func() {
		defer close(txsCh)

		var (
			transactions []*synctransaction.TransactionInBlock
			receipts     []*receipt.Receipt
		)

	loop:
		for res := range txsIt {
			switch v := res.TransactionMessage.(type) {
			case *synctransaction.TransactionsResponse_TransactionWithReceipt:
				txWithReceipt := v.TransactionWithReceipt
				transactions = append(transactions, txWithReceipt.Transaction)
				receipts = append(receipts, txWithReceipt.Receipt)
			case *synctransaction.TransactionsResponse_Fin:
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

func randomPeer(host host.Host, log utils.SimpleLogger) peer.ID {
	store := host.Peerstore()
	// todo do not request same block from all peers
	peers := utils.Filter(store.Peers(), func(peerID peer.ID) bool {
		return peerID != host.ID()
	})
	if len(peers) == 0 {
		return ""
	}

	p := peers[rand.Intn(len(peers))] //nolint:gosec

	log.Debugw("Number of peers", "len", len(peers))
	log.Debugw("Random chosen peer's info", "peerInfo", store.PeerInfo(p))

	return p
}

var errNoPeers = errors.New("no peers available")

func randomPeerStream(host host.Host, log utils.SimpleLogger) NewStreamFunc {
	return func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		randPeer := randomPeer(host, log)
		if randPeer == "" {
			return nil, errNoPeers
		}
		stream, err := host.NewStream(ctx, randPeer, pids...)
		if err != nil {
			log.Debugw("Error creating stream", "peer", randPeer, "err", err)
			removePeer(host, log, randPeer)
			return nil, err
		}
		return stream, err
	}
}

func removePeer(host host.Host, log utils.SimpleLogger, id peer.ID) {
	log.Debugw("Removing peer", "peerID", id)
	host.Peerstore().RemovePeer(id)
	host.Peerstore().ClearAddrs(id)
}

func (s *BlockFetcher) createIteratorForBlock(blockNumber uint64) *synccommon.Iteration {
	return &synccommon.Iteration{
		Start:     &synccommon.Iteration_BlockNumber{BlockNumber: blockNumber},
		Direction: synccommon.Iteration_Forward,
		Limit:     1,
		Step:      1,
	}
}

func (s *BlockFetcher) WithListener(l junoSync.EventListener) {
	s.listener = l
}

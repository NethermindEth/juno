package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/sourcegraph/conc/stream"
)

var (
	_ service.Service = (*Synchronizer)(nil)
	_ Reader          = (*Synchronizer)(nil)
)

const (
	OpVerify = "verify"
	OpStore  = "store"
	OpFetch  = "fetch"
)

// This is a work-around. mockgen chokes when the instantiated generic type is in the interface.
type HeaderSubscription struct {
	*feed.Subscription[*core.Header]
}

// Todo: Since this is also going to be implemented by p2p package we should move this interface to node package
//
//go:generate mockgen -destination=../mocks/mock_synchronizer.go -package=mocks -mock_names Reader=MockSyncReader github.com/NethermindEth/juno/sync Reader
type Reader interface {
	StartingBlockNumber() (uint64, error)
	HighestBlockHeader() *core.Header
	SubscribeNewHeads() HeaderSubscription
}

// This is temporary and will be removed once the p2p synchronizer implements this interface.
type NoopSynchronizer struct{}

func (n *NoopSynchronizer) StartingBlockNumber() (uint64, error) {
	return 0, nil
}

func (n *NoopSynchronizer) HighestBlockHeader() *core.Header {
	return nil
}

func (n *NoopSynchronizer) SubscribeNewHeads() HeaderSubscription {
	return HeaderSubscription{feed.New[*core.Header]().Subscribe()}
}

// Synchronizer manages a list of StarknetData to fetch the latest blockchain updates
type Synchronizer struct {
	blockchain          *blockchain.Blockchain
	client              *http.Client
	starknetData        starknetdata.StarknetData
	startingBlockNumber *uint64
	highestBlockHeader  atomic.Pointer[core.Header]
	newHeads            *feed.Feed[*core.Header]
	latestBlockHeight   uint64
	log                 utils.SimpleLogger
	listener            EventListener

	retryInterval       time.Duration // Retry interval when reached the head
	pendingPollInterval time.Duration
	catchUpMode         bool
}

func New(bc *blockchain.Blockchain, starkNetData starknetdata.StarknetData,
	logger utils.SimpleLogger, pendingPollInterval time.Duration,
) *Synchronizer {
	s := &Synchronizer{
		blockchain:          bc,
		client:              http.DefaultClient,
		starknetData:        starkNetData,
		log:                 logger,
		newHeads:            feed.New[*core.Header](),
		pendingPollInterval: pendingPollInterval,
		listener:            &SelectiveListener{},
		latestBlockHeight:   uint64(0),
		retryInterval:       5 * time.Second,
	}
	return s
}

// WithListener registers an EventListener
func (s *Synchronizer) WithListener(listener EventListener) *Synchronizer {
	s.listener = listener
	return s
}

// Run starts the Synchronizer, returns an error if the loop is already running
func (s *Synchronizer) Run(ctx context.Context) error {
	s.syncBlocksFeeder(ctx)
	return nil
}

func (s *Synchronizer) fetchUnknownClasses(ctx context.Context, stateUpdate *core.StateUpdate) (map[felt.Felt]core.Class, error) {
	state, closer, err := s.blockchain.HeadState()
	if err != nil {
		// if err is db.ErrKeyNotFound we are on an empty DB
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
		closer = func() error {
			return nil
		}
	}

	newClasses := make(map[felt.Felt]core.Class)
	fetchIfNotFound := func(classHash *felt.Felt) error {
		if _, ok := newClasses[*classHash]; ok {
			return nil
		}

		stateErr := db.ErrKeyNotFound
		if state != nil {
			_, stateErr = state.Class(classHash)
		}

		if errors.Is(stateErr, db.ErrKeyNotFound) {
			class, fetchErr := s.starknetData.Class(ctx, classHash)
			if fetchErr == nil {
				newClasses[*classHash] = class
			}
			return fetchErr
		}
		return stateErr
	}

	for _, classHash := range stateUpdate.StateDiff.DeployedContracts {
		if err = fetchIfNotFound(classHash); err != nil {
			return nil, utils.RunAndWrapOnError(closer, err)
		}
	}
	for _, classHash := range stateUpdate.StateDiff.DeclaredV0Classes {
		if err = fetchIfNotFound(classHash); err != nil {
			return nil, utils.RunAndWrapOnError(closer, err)
		}
	}
	for classHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		if err = fetchIfNotFound(&classHash); err != nil {
			return nil, utils.RunAndWrapOnError(closer, err)
		}
	}

	return newClasses, closer()
}

func (s *Synchronizer) syncBlocksFeeder(syncCtx context.Context) {
	streamCtx, streamCancel := context.WithCancel(syncCtx)
	for {
		select {
		case <-streamCtx.Done():
			streamCancel()
			return
		default:
			s.log.Infow("Fetching from feeder")
			block, stateUpdate, blockCommitments := s.getBlockNumberDetails(s.latestBlockHeight)
			newClass, _ := s.fetchUnknownClasses(syncCtx, &stateUpdate)
			s.verifierTask(streamCtx, &block, &stateUpdate, newClass, streamCancel)
			err := s.blockchain.Store(&block, &blockCommitments, &stateUpdate, newClass)
			if err != nil {
				s.log.Errorw("Error storing the block% v", err)
			} else {
				s.log.Infow("Stored Block", "number", block.Number, "hash", block.Hash.ShortString(), "root", block.GlobalStateRoot.ShortString())
			}
			s.latestBlockHeight += 1
		}
	}
}

func (s *Synchronizer) verifierTask(ctx context.Context, block *core.Block, stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.Class, resetStreams context.CancelFunc,
) stream.Callback {
	verifyTimer := time.Now()
	commitments, err := s.blockchain.SanityCheckNewHeight(block, stateUpdate, newClasses)
	if err == nil {
		s.listener.OnSyncStepDone(OpVerify, block.Number, time.Since(verifyTimer))
	}
	return func() {
		select {
		case <-ctx.Done():
			return
		default:
			if err != nil {
				s.log.Warnw("Sanity checks failed", "number", block.Number, "hash", block.Hash.ShortString(), "err", err)
				resetStreams()
				return
			}
			storeTimer := time.Now()
			err = s.blockchain.Store(block, commitments, stateUpdate, newClasses)
			if err != nil {
				if errors.Is(err, blockchain.ErrParentDoesNotMatchHead) {
					// revert the head and restart the sync process, hoping that the reorg is not deep
					// if the reorg is deeper, we will end up here again and again until we fully revert reorged
					// blocks
					s.revertHead(block)
				} else {
					s.log.Warnw("Failed storing Block", "number", block.Number,
						"hash", block.Hash.ShortString(), "err", err)
				}
				resetStreams()
				return
			}
			s.listener.OnSyncStepDone(OpStore, block.Number, time.Since(storeTimer))

			highestBlockHeader := s.highestBlockHeader.Load()
			if highestBlockHeader != nil {
				isBehind := highestBlockHeader.Number > block.Number+uint64(maxWorkers())
				if s.catchUpMode != isBehind {
					resetStreams()
				}
				s.catchUpMode = isBehind
			}

			if highestBlockHeader == nil || highestBlockHeader.Number < block.Number {
				s.highestBlockHeader.CompareAndSwap(highestBlockHeader, block.Header)
			}

			s.newHeads.Send(block.Header)
			s.log.Infow("Stored Block", "number", block.Number, "hash",
				block.Hash.ShortString(), "root", block.GlobalStateRoot.ShortString())
		}
	}
}

func maxWorkers() int {
	m, mProcs := 16, runtime.GOMAXPROCS(0)
	if mProcs > m {
		return m
	}
	return mProcs
}

func (s *Synchronizer) revertHead(forkBlock *core.Block) {
	var localHead *felt.Felt
	head, err := s.blockchain.HeadsHeader()
	if err == nil {
		localHead = head.Hash
	}

	s.log.Infow("Reorg detected", "localHead", localHead, "forkHead", forkBlock.Hash)

	err = s.blockchain.RevertHead()
	if err != nil {
		s.log.Warnw("Failed reverting HEAD", "reverted", localHead, "err", err)
	} else {
		s.log.Infow("Reverted HEAD", "reverted", localHead)
	}
	s.listener.OnReorg(head.Number)
}

func (s *Synchronizer) getBlockNumberDetails(blockNumber uint64) (core.Block, core.StateUpdate, core.BlockCommitments) {
	block, commitments := s.getBlockFeederGateway(blockNumber)
	stateUpdate := s.getStateUpdate(blockNumber)
	return block, stateUpdate, commitments
}

func (s *Synchronizer) getBlockFeederGateway(blockNumber uint64) (core.Block, core.BlockCommitments) {
	getBlockURL := fmt.Sprintf("https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=%d", blockNumber)
	blockResponse, err := http.Get(getBlockURL)
	if err != nil {
		s.log.Errorw("Failed to get response: %v", err)
	}
	defer blockResponse.Body.Close()
	var block starknet.Block
	if blockResponse.StatusCode == http.StatusOK {
		decoder := json.NewDecoder(blockResponse.Body)

		if err := decoder.Decode(&block); err != nil {
			s.log.Errorw("Failed to decode response: %v", err)
		}
	}

	var adaptedTransactions []core.Transaction
	for _, transactionFromBlock := range block.Transactions {
		adaptedTransaction, _ := sn2core.AdaptTransaction(transactionFromBlock)
		adaptedTransactions = append(adaptedTransactions, adaptedTransaction)
	}

	var eventCount uint64 = 0
	var adaptedTransactionReceipt []*core.TransactionReceipt
	for _, receipt := range block.Receipts {
		eventCount += uint64(len(receipt.Events))
		adaptedTransaction := *sn2core.AdaptTransactionReceipt(receipt)
		adaptedTransactionReceipt = append(adaptedTransactionReceipt, &adaptedTransaction)
	}

	header := s.buildHeaderGateway(blockNumber, &block, eventCount, core.EventsBloom(adaptedTransactionReceipt))
	return core.Block{
			Header:       &header,
			Transactions: adaptedTransactions,
			Receipts:     adaptedTransactionReceipt,
		},
		core.BlockCommitments{
			TransactionCommitment: block.TransactionCommitment,
			EventCommitment:       block.EventCommitment,
		}
}

func (s *Synchronizer) buildHeaderGateway(
	blockNumber uint64,
	block *starknet.Block,
	eventCount uint64,
	eventsBloom *bloom.BloomFilter,
) core.Header {
	getSignatureURL := fmt.Sprintf("https://alpha-mainnet.starknet.io/feeder_gateway/get_signature?blockNumber=%d", blockNumber)
	signatureResponse, sigErr := http.Get(getSignatureURL)
	if sigErr != nil {
		log.Fatalf("Failed to get response: %v", sigErr)
	}
	var signature starknet.Signature
	if signatureResponse.StatusCode == http.StatusOK {
		decoder := json.NewDecoder(signatureResponse.Body)

		if err := decoder.Decode(&signature); err != nil {
			log.Fatalf("Failed to decode response: %v", err)
		}
	}
	defer signatureResponse.Body.Close()

	sigs := buildSignature(&signature)

	return core.Header{
		Hash:             block.Hash,
		ParentHash:       block.ParentHash,
		Number:           block.Number,
		GlobalStateRoot:  block.StateRoot,
		SequencerAddress: block.SequencerAddress,
		TransactionCount: uint64(len(block.Receipts)),
		EventCount:       eventCount,
		Timestamp:        block.Timestamp,
		ProtocolVersion:  block.Version,
		EventsBloom:      eventsBloom,
		GasPrice:         block.GasPriceETH(),
		GasPriceSTRK:     block.GasPriceSTRK(),
		L1DAMode:         core.L1DAMode(block.L1DAMode),
		L1DataGasPrice:   (*core.GasPrice)(block.L1DataGasPrice),
		Signatures:       sigs,
	}
}

func buildSignature(sig *starknet.Signature) [][]*felt.Felt {
	sigs := [][]*felt.Felt{}
	if sig != nil {
		sigs = append(sigs, sig.Signature)
	}
	return sigs
}

func (s *Synchronizer) getStateUpdate(blockNumber uint64) core.StateUpdate {
	stateUpdateURL := fmt.Sprintf("https://alpha-mainnet.starknet.io/feeder_gateway/get_state_update?blockNumber=%d", blockNumber)
	response, err := http.Get(stateUpdateURL)
	if err != nil {
		s.log.Errorw("Failed to get response: %v", err)
	}
	var stateUpdateJSON core.StateUpdateJSON
	if response.StatusCode == http.StatusOK {
		decoder := json.NewDecoder(response.Body)

		if err := decoder.Decode(&stateUpdateJSON); err != nil {
			s.log.Errorw("Failed to decode response: %v", err)
		}
	}
	stateUpdate := core.StateUpdateAdapter(stateUpdateJSON)

	return stateUpdate
}

func (s *Synchronizer) StartingBlockNumber() (uint64, error) {
	if s.startingBlockNumber == nil {
		return 0, errors.New("not running")
	}
	return *s.startingBlockNumber, nil
}

func (s *Synchronizer) HighestBlockHeader() *core.Header {
	return s.highestBlockHeader.Load()
}

func (s *Synchronizer) SubscribeNewHeads() HeaderSubscription {
	return HeaderSubscription{
		Subscription: s.newHeads.Subscribe(),
	}
}

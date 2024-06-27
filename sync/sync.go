package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	readOnlyBlockchain  bool
	starknetData        starknetdata.StarknetData
	startingBlockNumber *uint64
	highestBlockHeader  atomic.Pointer[core.Header]
	newHeads            *feed.Feed[*core.Header]
	latestBlockHeight   uint64
	log                 utils.SimpleLogger
	listener            EventListener
	pendingPollInterval time.Duration
	timeoutDuration     time.Duration
}

func New(bc *blockchain.Blockchain, starkNetData starknetdata.StarknetData,
	log utils.SimpleLogger, pendingPollInterval time.Duration, readOnlyBlockchain bool,
) *Synchronizer {
	head, err := bc.Head()
	latestBlockHeight := uint64(0)
	if err == nil {
		latestBlockHeight = head.Number + uint64(1)
	}

	s := &Synchronizer{
		blockchain:          bc,
		starknetData:        starkNetData,
		log:                 log,
		newHeads:            feed.New[*core.Header](),
		latestBlockHeight:   latestBlockHeight,
		pendingPollInterval: pendingPollInterval,
		listener:            &SelectiveListener{},
		readOnlyBlockchain:  readOnlyBlockchain,
		timeoutDuration:     10 * time.Second,
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
	s.syncBlocksFromFeederGateway(ctx)
	return nil
}

func (s *Synchronizer) syncBlocksFromFeederGateway(syncCtx context.Context) {
	streamCtx, streamCancel := context.WithCancel(syncCtx)
	for {
		select {
		case <-streamCtx.Done():
			streamCancel()
			return
		default:
			s.log.Infow("Fetching from feeder gateway")
			block, stateUpdate, blockCommitments := s.getBlockDetails(s.latestBlockHeight)
			newClasses, err := s.fetchUnknownClasses(syncCtx, &stateUpdate)
			if err != nil {
				s.log.Errorw("Error fetching unknown classes: %v", err)
			}
			err = s.blockchain.Store(&block, &blockCommitments, &stateUpdate, newClasses)
			if err != nil {
				s.log.Errorw("Error storing block: %v", err)
			} else {
				s.log.Infow("Stored Block", "number", block.Number, "hash",
					block.Hash.ShortString(), "root", block.GlobalStateRoot.ShortString())
			}
			s.latestBlockHeight += 1
		}
	}
}

func (s *Synchronizer) getBlockDetails(blockNumber uint64) (core.Block, core.StateUpdate, core.BlockCommitments) {
	block, blockCommitments := s.getBlockFromFeederGateway(blockNumber)
	stateUpdate := s.getStateUpdateFromFeederGateway(blockNumber)
	return block, stateUpdate, blockCommitments
}

//nolint:dupl
func (s *Synchronizer) constructBlock(blockNumber uint64) (starknet.Block, error) {
	blockURL := fmt.Sprintf("https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=%d", blockNumber)
	parsedBlockURL, err := url.Parse(blockURL)
	if err != nil {
		return starknet.Block{}, fmt.Errorf("invalid URL: %v", err)
	}

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), s.timeoutDuration)
	defer cancel()

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, "GET", parsedBlockURL.String(), http.NoBody)
	if err != nil {
		return starknet.Block{}, fmt.Errorf("error creating request: %v", err)
	}

	client := &http.Client{}
	blockResponse, err := client.Do(req)
	if err != nil {
		return starknet.Block{}, fmt.Errorf("error getting response: %v", err)
	}
	defer blockResponse.Body.Close()

	var block starknet.Block
	if blockResponse.StatusCode == http.StatusOK {
		decoder := json.NewDecoder(blockResponse.Body)
		if errDecode := decoder.Decode(&block); errDecode != nil {
			return starknet.Block{}, fmt.Errorf("failed to decode response: %v", errDecode)
		}
	} else {
		return starknet.Block{}, fmt.Errorf("received non-OK HTTP status: %v", blockResponse.Status)
	}

	return block, nil
}

//nolint:dupl
func (s *Synchronizer) constructSignature(blockNumber uint64) (starknet.Signature, error) {
	signatureURL := fmt.Sprintf("https://alpha-mainnet.starknet.io/feeder_gateway/get_signature?blockNumber=%d", blockNumber)
	parsedSignatureURL, err := url.Parse(signatureURL)
	if err != nil {
		return starknet.Signature{}, fmt.Errorf("invalid URL: %v", err)
	}

	// Create a context with the timeout duration
	ctx, cancel := context.WithTimeout(context.Background(), s.timeoutDuration)
	defer cancel()

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, "GET", parsedSignatureURL.String(), http.NoBody)
	if err != nil {
		return starknet.Signature{}, fmt.Errorf("error creating request: %v", err)
	}

	client := &http.Client{}
	signatureResponse, err := client.Do(req)
	if err != nil {
		return starknet.Signature{}, fmt.Errorf("error getting response: %v", err)
	}
	defer signatureResponse.Body.Close()

	var signature starknet.Signature
	if signatureResponse.StatusCode == http.StatusOK {
		decoder := json.NewDecoder(signatureResponse.Body)
		if err := decoder.Decode(&signature); err != nil {
			return starknet.Signature{}, fmt.Errorf("failed to decode response: %v", err)
		}
	} else {
		return starknet.Signature{}, fmt.Errorf("received non-OK HTTP status: %v", signatureResponse.Status)
	}

	return signature, nil
}

func (s *Synchronizer) getBlockFromFeederGateway(blockNumber uint64) (core.Block, core.BlockCommitments) {
	block, blockErr := s.constructBlock(blockNumber)
	if blockErr != nil {
		s.log.Errorw(blockErr.Error())
		return core.Block{}, core.BlockCommitments{}
	}

	signature, signatureErr := s.constructSignature(blockNumber)
	if signatureErr != nil {
		s.log.Errorw(signatureErr.Error())
		return core.Block{}, core.BlockCommitments{}
	}

	var adaptedTransactions []core.Transaction
	for _, transaction := range block.Transactions {
		adaptedTransaction, err := sn2core.AdaptTransaction(transaction)
		if err != nil {
			s.log.Errorw("Error adapting starknet type to core type: %v", err)
			return core.Block{}, core.BlockCommitments{}
		}
		adaptedTransactions = append(adaptedTransactions, adaptedTransaction)
	}

	var eventCount uint64
	var adaptedTransactionReceipts []*core.TransactionReceipt
	for _, receipt := range block.Receipts {
		eventCount += uint64(len(receipt.Events))
		adaptedTransactionReceipt := sn2core.AdaptTransactionReceipt(receipt)
		adaptedTransactionReceipts = append(adaptedTransactionReceipts, adaptedTransactionReceipt)
	}

	signatures := [][]*felt.Felt{}
	signatures = append(signatures, signature.Signature)

	header := &core.Header{
		Hash:             block.Hash,
		ParentHash:       block.ParentHash,
		Number:           block.Number,
		GlobalStateRoot:  block.StateRoot,
		SequencerAddress: block.SequencerAddress,
		TransactionCount: uint64(len(block.Transactions)),
		EventCount:       eventCount,
		Timestamp:        block.Timestamp,
		ProtocolVersion:  block.Version,
		EventsBloom:      core.EventsBloom(adaptedTransactionReceipts),
		GasPrice:         block.GasPriceETH(),
		Signatures:       signatures,
		GasPriceSTRK:     (*felt.Felt)(nil),
		L1DAMode:         core.L1DAMode(block.L1DAMode),
		L1DataGasPrice:   (*core.GasPrice)(nil),
	}

	return core.Block{
			Header:       header,
			Transactions: adaptedTransactions,
			Receipts:     adaptedTransactionReceipts,
		},
		core.BlockCommitments{
			TransactionCommitment: block.TransactionCommitment,
			EventCommitment:       block.EventCommitment,
		}
}

func (s *Synchronizer) getStateUpdateFromFeederGateway(blockNumber uint64) core.StateUpdate {
	stateUpdateURL := fmt.Sprintf("https://alpha-mainnet.starknet.io/feeder_gateway/get_state_update?blockNumber=%d", blockNumber)
	parsedStateUpdateURL, err := url.Parse(stateUpdateURL)
	if err != nil {
		s.log.Errorw("Invalid URL: %v", err)
		return core.StateUpdate{}
	}

	// Create a context with the timeout duration
	ctx, cancel := context.WithTimeout(context.Background(), s.timeoutDuration)
	defer cancel()

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, "GET", parsedStateUpdateURL.String(), http.NoBody)
	if err != nil {
		s.log.Errorw("Error creating request: %v", err)
		return core.StateUpdate{}
	}

	client := &http.Client{}
	stateUpdateResponse, err := client.Do(req)
	if err != nil {
		s.log.Errorw("Error getting response: %v", err)
		return core.StateUpdate{}
	}
	defer stateUpdateResponse.Body.Close()

	var stateUpdate starknet.StateUpdate
	if stateUpdateResponse.StatusCode == http.StatusOK {
		decoder := json.NewDecoder(stateUpdateResponse.Body)
		if errDecode := decoder.Decode(&stateUpdate); errDecode != nil {
			s.log.Errorw("Failed to decode response: %v", errDecode)
			return core.StateUpdate{}
		}
	} else {
		s.log.Warnw("Received non-OK HTTP status: %v", stateUpdateResponse.Status)
		return core.StateUpdate{}
	}

	adaptedStateUpdate, err := sn2core.AdaptStateUpdate(&stateUpdate)
	if err != nil {
		s.log.Errorw("Error adapting starknet type to core type: %v", err)
		return core.StateUpdate{}
	}

	return *adaptedStateUpdate
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

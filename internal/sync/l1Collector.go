package sync

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/internal/log"

	"github.com/NethermindEth/juno/internal/sync/abi"

	"github.com/NethermindEth/juno/pkg/felt"
	types2 "github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/ethereum/go-ethereum"
	ethAbi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ErrorMemoryPageNotFound = errors.New("memory page not found")
	ErrorFactAlreadySaved   = errors.New("fact already saved")
)

var windowSize = 5_000

type l1Collector struct {
	logger log.Logger
	// manager is the Sync manager
	manager *sync.Manager
	// l1Client is the client that will be used to fetch the data that comes from the Ethereum Node.
	l1Client L1Client
	// client is the client that will be used to fetch the data that comes from the Feeder Gateway.
	client *feeder.Client
	// chainID represent the chain id of the node.
	chainID int
	// latestBlockSynced is the last block that was synced.
	latestBlockSynced int64
	// buffer is the channel that will be used to collect the StateDiff.
	buffer chan *CollectorDiff

	// starknetContractAddress is the address of the Starknet contract on Layer 1.
	starknetContractAddress common.Address
	// GpsVerifierContractAddress is the address of the GpsVerifier contract on Layer 1.
	gpsVerifierContractAddress common.Address
	// MemoryPagesContractAddress is the address of the MemoryPages contract on Layer 1.
	memoryPagesContractAddress common.Address
	// StarkNetABI is the ABI of the Starknet contract on Layer 1.
	starknetABI ethAbi.ABI

	// contractInfo is the information of the contracts on Layer 1.
	contractInfo map[common.Address]types2.ContractInfo

	// factInformation
	memoryPageHash *types2.Dictionary
	gpsVerifier    *types2.Dictionary
	facts          *types2.Dictionary

	// latestBlock is the last block that was synced.
	latestBlock *feeder.StarknetBlock
	// pendingBlock is the block that is being synced.
	pendingBlock *feeder.StarknetBlock

	quit chan struct{}
}

// NewL1Collector creates a new Ethereum Collector.
// notest
func NewL1Collector(manager *sync.Manager, feeder *feeder.Client, l1client L1Client, chainID int, logger log.Logger) *l1Collector {
	collector := &l1Collector{
		client:   feeder,
		manager:  manager,
		chainID:  chainID,
		l1Client: l1client,
		quit:     make(chan struct{}),
	}
	collector.logger = logger.Named("l1Collector")
	collector.buffer = make(chan *CollectorDiff, 10)
	collector.starknetABI, _ = loadAbiOfContract(abi.StarknetAbi)
	collector.memoryPageHash = types2.NewDictionary()
	collector.gpsVerifier = types2.NewDictionary()
	collector.facts = types2.NewDictionary()
	// load the contract info and retry if error up to 10 times.
	for i := 0; i < 10; i++ {
		err := collector.loadContractsAbi()
		if err == nil {
			break
		}
		time.Sleep(time.Second * 10)
	}
	go collector.updateLatestBlockOnChain()
	return collector
}

func (l *l1Collector) Run() {
	l.logger.Info("Collector Started")

	l.latestBlockSynced = l.manager.GetLatestBlockSync()

	go l.handleEvents()

	// start the buffer updater
	for {
		select {
		case <-l.quit:
			close(l.buffer)
			return
		default:
			if l.latestBlock == nil {
				time.Sleep(time.Second * 3)
				continue
			}
			if l.latestBlockSynced >= int64(l.latestBlock.BlockNumber) {
				time.Sleep(time.Second * 3)
				continue
			}

			if !l.facts.Exist(strconv.FormatInt(l.latestBlockSynced, 10)) {
				time.Sleep(time.Second * 3)
				continue
			}
			f, _ := l.facts.Get(strconv.FormatInt(l.latestBlockSynced, 10))
			fact := f.(*types2.Fact)

			if l.gpsVerifier.Exist(fact.Value) {
				// Get memory pages hashes using fact
				pagesHashes, ok := l.gpsVerifier.Get(fact.Value)
				if !ok {
					l.logger.Debug("Fact has not been verified")
					time.Sleep(time.Second * 3)
					continue
				}
				// If already exist the information related to the fact,
				// fetch the memory pages and updated the state
				pages, err := l.processPagesHashes(
					pagesHashes.(types2.PagesHash).Bytes,
					l.contractInfo[l.memoryPagesContractAddress].Contract)
				if err != nil {
					l.logger.Debugw("Error processing pages hashes", "error", err)
					time.Sleep(time.Second * 3)
					continue
				}

				stateDiff := parsePages(pages)
				stateDiff.NewRoot = fact.StateRoot
				stateDiff.BlockNumber = int64(fact.SequenceNumber)
				l.buffer <- fetchContractCode(stateDiff, l.client, l.logger)

				l.removeFactTree(fact)
				l.logger.Infow("StateUpdate collected", "BlockNumber", l.latestBlockSynced)
				l.latestBlockSynced += 1
			}

		}
	}
}

func (l *l1Collector) GetChannel() chan *CollectorDiff {
	return l.buffer
}

func (l *l1Collector) Close() {
	close(l.quit)
}

func (l *l1Collector) LatestBlock() *feeder.StarknetBlock {
	return l.latestBlock
}

func (l *l1Collector) PendingBlock() *feeder.StarknetBlock {
	return l.pendingBlock
}

func (l *l1Collector) updateLatestBlockOnChain() {
	go l.updatePendingBlock()
	number, err := l.l1Client.BlockNumber(context.Background())
	if err != nil {
		l.logger.Error("Error subscribing to logs", "err", err)
		return
	}
	// build query for the latest block
	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetInt64(int64(number) - int64(10*windowSize)),
		Addresses: []common.Address{l.starknetContractAddress},
		Topics:    [][]common.Hash{{crypto.Keccak256Hash([]byte(l.starknetABI.Events["LogStateUpdate"].Sig))}},
	}
	logs, err := l.l1Client.FilterLogs(context.Background(), query)
	if err != nil {
		l.logger.Error("Error subscribing to logs", "err", err)
		return
	}
	for _, logFetched := range logs {
		l.updateBlockOnChain(logFetched.Data)
	}

	subLogs := make(chan types.Log)
	subscription, err := l.l1Client.SubscribeFilterLogs(context.Background(), query, subLogs)
	defer subscription.Unsubscribe()

	for logFetched := range subLogs {
		l.updateBlockOnChain(logFetched.Data)
	}
}

// processPagesHashes takes an array of arrays of pages' hashes and
// converts them into memory pages by querying an ethereum client.
// notest
func (l *l1Collector) processPagesHashes(pagesHashes [][32]byte, memoryContract ethAbi.ABI) ([][]*big.Int, error) {
	pages := make([][]*big.Int, 0)
	for _, v := range pagesHashes {
		// Get transactionsHash based on the memory page
		hash := common.BytesToHash(v[:])
		transactionHash, ok := l.memoryPageHash.Get(hash)
		if !ok {
			return nil, ErrorMemoryPageNotFound
		}
		txHash := transactionHash.(types2.TxnHash).Hash
		txn, _, err := l.l1Client.TransactionByHash(context.Background(), txHash)
		if err != nil {
			l.logger.Errorw("Couldn't retrieve transactions", "error", err, "Transaction Hash", v)
			return nil, err
		}

		// Parse Ethereum transaction calldata for Starknet transaction information
		data := txn.Data()[4:] // Remove the method signature hash
		inputs := make(map[string]interface{})
		err = memoryContract.Methods["registerContinuousMemoryPage"].Inputs.UnpackIntoMap(inputs, data)
		if err != nil {
			l.logger.Infow("Couldn't unpack into map", "error", err)
			return nil, err
		}
		// Append calldata to pages
		pages = append(pages, inputs["values"].([]*big.Int))
	}
	return pages, nil
}

func (l *l1Collector) loadContractsAbi() error {
	l.contractInfo = make(map[common.Address]types2.ContractInfo)

	contractAddresses, err := l.client.GetContractAddresses()
	if err != nil {
		l.logger.Debugw("Couldn't get ContractInfo Address from Feeder Gateway", "error", err)
		return err
	}

	// Add Starknet contract
	err = loadContractInfo(contractAddresses.Starknet,
		abi.StarknetAbi,
		"LogStateTransitionFact", l.contractInfo)
	if err != nil {
		l.logger.Debugw("Couldn't load contract from disk ", "Address", contractAddresses.Starknet)
		return err
	}
	l.starknetContractAddress = common.HexToAddress(contractAddresses.Starknet)

	// Add Gps Statement Verifier contract
	gpsAddress := getGpsVerifierContractAddress(l.chainID)
	err = loadContractInfo(gpsAddress,
		abi.GpsVerifierAbi,
		"LogMemoryPagesHashes", l.contractInfo)
	if err != nil {
		l.logger.Debugw("Couldn't load contract from disk", "Address", gpsAddress)
		return err
	}
	l.gpsVerifierContractAddress = common.HexToAddress(gpsAddress)

	// Add Memory Page Fact Registry contract
	memoryPagesContractAddress := getMemoryPagesContractAddress(l.chainID)
	err = loadContractInfo(memoryPagesContractAddress,
		abi.MemoryPagesAbi,
		"LogMemoryPageFactContinuous", l.contractInfo)
	if err != nil {
		l.logger.Debugw("Couldn't load contract from disk ", "Address", memoryPagesContractAddress)
		return err
	}
	l.memoryPagesContractAddress = common.HexToAddress(memoryPagesContractAddress)
	return nil
}

func (l *l1Collector) handleEvents() {
	// Get the block in which we are going to start processing events
	// - 100 blocks back just in case.
	initialBlock := l.manager.GetBlockOfProcessedEvent(l.latestBlockSynced) - int64(windowSize)

	initialBlock = int64(math.Max(float64(initialBlock), float64(initialBlockForStarknetContract(l.chainID))))
	blockNumber, err := l.l1Client.BlockNumber(context.Background())
	if err != nil {
		l.logger.Error("Error fetching latest block on Ethereum", "err", err)
		return
	}

	// Keep updated the blockNumber of the latest block on L1
	go func() {
		blockNumber, err = l.l1Client.BlockNumber(context.Background())
		if err != nil {
			l.logger.Error("Error fetching latest block on Ethereum", "err", err)
		}
		time.Sleep(time.Second * 5)
	}()
	// Process blocks until latest
	for initialBlock < int64(blockNumber) {
		if len(l.buffer) > 8 {
			l.logger.Info("Buffer contains some elements, waiting to get more events")
			time.Sleep(10 * time.Second)
			continue
		}
		err = l.processBatchOfEvents(initialBlock, int64(windowSize))
		if err != nil {
			l.logger.Error("Error processing batch of events", "err", err)
			return
		}
		initialBlock += int64(windowSize)
	}

	// Subscribe for new blocks
	err = l.processSubscription(initialBlock)
	if err != nil {
		l.logger.Errorw("Error subscribing to events", "error", err)
		return
	}
}

// processSubscription iterates over the logs that has been thrown while subscribed to the L1
func (l *l1Collector) processSubscription(initialBlock int64) error {
	addresses := make([]common.Address, 0)

	topics := make([]common.Hash, 0)
	for k, v := range l.contractInfo {
		addresses = append(addresses, k)
		topics = append(topics, crypto.Keccak256Hash([]byte(v.Contract.Events[v.EventName].Sig)))
	}
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(initialBlock),
		Addresses: addresses,
	}
	hLog := make(chan types.Log)
	sub, err := l.l1Client.SubscribeFilterLogs(context.Background(), query, hLog)
	if err != nil {
		l.logger.Info("Couldn't subscribe for incoming blocks")
		return err
	}
	for {
		select {
		case err = <-sub.Err():
			l.logger.Debugw("Error getting the latest logs", "error", err)
		case vLog := <-hLog:
			event := map[string]interface{}{}
			err = l.contractInfo[vLog.Address].Contract.UnpackIntoMap(event, l.contractInfo[vLog.Address].EventName, vLog.Data)
			if err != nil {
				l.logger.Debugw("Couldn't get event from log", "error", err)
				continue
			}
			eventChan := &types2.EventInfo{
				Block:              vLog.BlockNumber,
				Event:              event,
				Address:            l.contractInfo[vLog.Address].Address,
				TxnHash:            vLog.TxHash,
				InitialBlockLogged: int64(vLog.BlockNumber),
			}

			l.processEvents(eventChan)

		}
	}
}

// processBatchOfEvents iterates over a batch of events and extracts the EventInfo
func (l *l1Collector) processBatchOfEvents(initialBlock, window int64) error {
	addresses := make([]common.Address, 0)

	topics := make([]common.Hash, 0)
	for k, v := range l.contractInfo {
		addresses = append(addresses, k)
		topics = append(topics, crypto.Keccak256Hash([]byte(v.Contract.Events[v.EventName].Sig)))
	}

	l.logger.Infow("Fetching logs....", "From Block", initialBlock, "To Block", initialBlock+window)
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(initialBlock),
		ToBlock:   big.NewInt(initialBlock + window),
		Addresses: addresses,
		Topics:    [][]common.Hash{topics},
	}

	starknetLogs, err := l.l1Client.FilterLogs(context.Background(), query)
	if err != nil {
		l.logger.Infow("Couldn't get logs", "error", err, "Initial block", initialBlock, "End block", initialBlock+window, "Addresses", addresses)
		return err
	}
	for _, vLog := range starknetLogs {
		event := map[string]interface{}{}
		// l.logger.With("Event Name", l.contractInfo[vLog.Address].EventName,
		//	"Contract Address", vLog.Address.Hex(), "Topics", vLog.Topics,
		//	"Address", addresses).Info("Unpacking event")

		err = l.contractInfo[vLog.Address].Contract.UnpackIntoMap(event, l.contractInfo[vLog.Address].EventName, vLog.Data)
		if err != nil {
			l.logger.Infow("Couldn't get LogStateTransitionFact from event", "error", err)
			continue
		}
		eventValue := &types2.EventInfo{
			Block:              vLog.BlockNumber,
			Event:              event,
			Address:            l.contractInfo[vLog.Address].Address,
			TxnHash:            vLog.TxHash,
			InitialBlockLogged: initialBlock,
		}
		l.processEvents(eventValue)
	}
	return nil
}

func (l *l1Collector) processEvents(event *types2.EventInfo) {
	// Process GpsStatementVerifier contract
	factHash, ok := event.Event["factHash"]
	pagesHashes, ok1 := event.Event["pagesHashes"]

	if ok && ok1 {
		b := make([]byte, 0)
		for _, v := range factHash.([32]byte) {
			b = append(b, v)
		}
		value := types2.PagesHash{Bytes: pagesHashes.([][32]byte)}
		l.gpsVerifier.Add(common.BytesToHash(b), value)
	}
	// Process MemoryPageFactRegistry contract
	if memoryHash, ok := event.Event["memoryHash"]; ok {
		key := common.BytesToHash(memoryHash.(*big.Int).Bytes())
		value := types2.TxnHash{Hash: event.TxnHash}
		l.memoryPageHash.Add(key, value)
	}
	// Process Starknet logs
	if fact, ok := event.Event["stateTransitionFact"]; ok {

		b := make([]byte, 0)
		for _, v := range fact.([32]byte) {
			b = append(b, v)
		}
		blockNumber := new(big.Int).SetUint64(event.Block)
		query := ethereum.FilterQuery{
			FromBlock: blockNumber,
			ToBlock:   blockNumber,
			Addresses: []common.Address{l.starknetContractAddress},
			Topics:    [][]common.Hash{{crypto.Keccak256Hash([]byte(l.starknetABI.Events["LogStateUpdate"].Sig))}},
		}

		starknetLogs, err := l.l1Client.FilterLogs(context.Background(), query)
		if err != nil {
			l.logger.Infow("Couldn't get logs", "error", err, "Initial block", event.Block, "End block", event.Block+1)
		}
		fullFact, err := l.getFactInfo(starknetLogs, common.BytesToHash(b), event.TxnHash)
		if err != nil {
			l.logger.Infow("Couldn't get fact info", "error", err)
			return
		}

		// Safe Fact for block x
		l.facts.Add(strconv.FormatUint(fullFact.SequenceNumber, 10), fullFact)
		l.manager.StoreBlockOfProcessedEvent(int64(fullFact.SequenceNumber), event.InitialBlockLogged)
	}
}

// getFactInfo gets the state root and sequence number associated with
// a given StateTransitionFact.
// notest
func (l *l1Collector) getFactInfo(starknetLogs []types.Log, fact common.Hash, txHash common.Hash) (*types2.Fact, error) {
	for _, vLog := range starknetLogs {
		event := map[string]interface{}{}
		err := l.starknetABI.UnpackIntoMap(event, "LogStateUpdate", vLog.Data)
		if err != nil {
			// notest
			l.logger.Infow("Couldn't get state root or sequence number from LogStateUpdate event", "error", err)
			continue
		}
		// Corresponding LogStateUpdate for the LogStateTransitionFact (they must occur in the same transaction)
		if vLog.TxHash.Hex() == txHash.Hex() {
			sequenceNumber := event["blockNumber"].(*big.Int).Uint64()
			_, ok := l.facts.Get(strconv.FormatUint(sequenceNumber, 10))
			if ok {
				// notest
				return nil, ErrorFactAlreadySaved
			} else {
				return &types2.Fact{
					StateRoot:      new(felt.Felt).SetBigInt(event["globalRoot"].(*big.Int)),
					SequenceNumber: sequenceNumber,
					Value:          fact,
				}, nil
			}
		}
	}
	// notest
	panic("Couldn't find a block number that match in the logs for given fact")
}

func (l *l1Collector) removeFactTree(fact *types2.Fact) {
	// Load GpsStatementVerifier from fact
	pagesHashes, _ := l.gpsVerifier.Get(fact.Value)

	if pagesHashes != nil {
		realPagesHashes := pagesHashes.(types2.PagesHash)

		// remove pages from MemoryPageFactRegistry
		for _, v := range realPagesHashes.Bytes {
			// Get transactionsHash based on the memory page
			hash := common.BytesToHash(v[:])
			l.memoryPageHash.Remove(hash)
		}

		// remove fact from GpsStatementVerifier
		l.gpsVerifier.Remove(fact.Value)
	}
	// remove Fact from facts
	l.facts.Remove(strconv.FormatUint(fact.SequenceNumber, 10))
}

func (l *l1Collector) updateBlockOnChain(logStateUpdateData []byte) {
	event := map[string]interface{}{}
	err := l.starknetABI.UnpackIntoMap(event, "LogStateUpdate", logStateUpdateData)
	if err != nil {
		l.logger.Infow("Couldn't get state root or sequence number from LogStateUpdate event", "error", err)
		return
	}
	// Corresponding LogStateUpdate for the LogStateTransitionFact (they must occur in the same transaction)
	sequenceNumber := event["blockNumber"].(*big.Int).Uint64()
	// extract the block number from the log
	latestBlockAcceptedOnL1, err := l.client.GetBlock("", strconv.FormatUint(sequenceNumber, 10))
	if err != nil {
		return
	}

	l.latestBlock = latestBlockAcceptedOnL1
}

func (l *l1Collector) updatePendingBlock() {
	l.fetchPendingBlock()
	for {
		time.Sleep(time.Minute)
	}
}

func (l *l1Collector) fetchPendingBlock() {
	pendingBlock, err := l.client.GetBlock("", "pending")
	if err != nil {
		return
	}
	if l.pendingBlock != nil && pendingBlock.ParentBlockHash == l.pendingBlock.ParentBlockHash {
		return
	}

	l.pendingBlock = pendingBlock
}

// parsePages converts an array of memory pages into a state diff that
// can be used to update the local state.
func parsePages(pages [][]*big.Int) *types2.StateUpdate {
	// Remove first page
	pagesWithoutFirst := pages[1:]

	// Flatter the pages recovered from Layer 1
	pagesFlatter := make([]*big.Int, 0)
	for _, page := range pagesWithoutFirst {
		pagesFlatter = append(pagesFlatter, page...)
	}

	// Get the number of contracts deployed in this block
	deployedContractsInfoLen := pagesFlatter[0].Int64()
	pagesFlatter = pagesFlatter[1:]
	deployedContracts := make([]types2.DeployedContract, 0)

	// Get the info of the deployed contracts
	deployedContractsData := pagesFlatter[:deployedContractsInfoLen]

	// Iterate while contains contract data to be processed
	for len(deployedContractsData) > 0 {
		// Parse the Address of the contract
		address := new(felt.Felt).SetBigInt(deployedContractsData[0])
		deployedContractsData = deployedContractsData[1:]

		// Parse the ContractInfo Hash
		contractHash := new(felt.Felt).SetBigInt(deployedContractsData[0])
		deployedContractsData = deployedContractsData[1:]

		// Parse the number of Arguments the constructor contains
		constructorArgumentsLen := deployedContractsData[0].Int64()
		deployedContractsData = deployedContractsData[1:]

		// Parse constructor arguments
		constructorArguments := make([]*felt.Felt, 0)
		for i := int64(0); i < constructorArgumentsLen; i++ {
			constructorArguments = append(constructorArguments, new(felt.Felt).SetBigInt(deployedContractsData[0]))
			deployedContractsData = deployedContractsData[1:]
		}

		// Store deployed ContractInfo information
		deployedContracts = append(deployedContracts, types2.DeployedContract{
			Address:             address,
			Hash:                contractHash,
			ConstructorCallData: constructorArguments,
		})
	}
	pagesFlatter = pagesFlatter[deployedContractsInfoLen:]

	// Parse the number of contracts updates
	numContractsUpdate := pagesFlatter[0].Int64()
	pagesFlatter = pagesFlatter[1:]

	storageDiff := make(types2.StorageDiff, 0)

	// Iterate over all the contracts that had been updated and collect the needed information
	for i := int64(0); i < numContractsUpdate; i++ {
		// Parse the Address of the contract
		address := new(felt.Felt).SetBigInt(pagesFlatter[0])
		pagesFlatter = pagesFlatter[1:]

		// Parse the number storage updates
		numStorageUpdates := pagesFlatter[0].Int64()
		pagesFlatter = pagesFlatter[1:]

		kvs := make([]types2.MemoryCell, 0)
		for k := int64(0); k < numStorageUpdates; k++ {
			kvs = append(kvs, types2.MemoryCell{
				Address: new(felt.Felt).SetBigInt(pagesFlatter[0]),
				Value:   new(felt.Felt).SetBigInt(pagesFlatter[1]),
			})
			pagesFlatter = pagesFlatter[2:]
		}
		storageDiff[*address] = kvs
	}

	return &types2.StateUpdate{
		DeployedContracts: deployedContracts,
		StorageDiff:       storageDiff,
	}
}

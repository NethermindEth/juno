package services

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/internal/services/abi"
	"github.com/NethermindEth/juno/pkg/felt"
	types2 "github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/internal/db/sync"
	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/ethereum/go-ethereum"
	ethAbi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var L1Collector *l1Collector

var (
	ErrorMemoryPageNotFound = errors.New("memory page not found")
	ErrorFactAlreadySaved   = errors.New("fact already saved")
)

var windowSize = 5_000

type l1Collector struct {
	service
	// manager is the Sync manager
	manager *sync.Manager
	// l1client is the client that will be used to fetch the data that comes from the Ethereum Node.
	l1client L1Client
	// client is the client that will be used to fetch the data that comes from the Feeder Gateway.
	client *feeder.Client
	// chainID represent the chain id of the node.
	chainID int
	// latestBlockSynced is the last block that was synced.
	latestBlockSynced int64
	// buffer is the channel that will be used to collect the StateDiff.
	buffer chan *types2.StateDiff

	// starknetContractAddress is the address of the Starknet contract on Layer 1.
	starknetContractAddress common.Address
	// GpsVerifierContractAddress is the address of the GpsVerifier contract on Layer 1.
	gpsVerifierContractAddress common.Address
	// MemoryPagesContractAddress is the address of the MemoryPages contract on Layer 1.
	memoryPagesContractAddress common.Address
	// StarkNetABI is the ABI of the Starknet contract on Layer 1.
	starknetABI ethAbi.ABI

	// Synced represent if the node has reach the synced status
	Synced bool

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
}

func NewL1Collector(manager *sync.Manager, feeder *feeder.Client, l1client L1Client, chainID int) {
	L1Collector = &l1Collector{
		client:   feeder,
		manager:  manager,
		chainID:  chainID,
		l1client: l1client,
	}
	L1Collector.logger = Logger.Named("l1Collector")
	L1Collector.buffer = make(chan *types2.StateDiff, 10)
	L1Collector.starknetABI, _ = loadAbiOfContract(abi.StarknetAbi)
	L1Collector.memoryPageHash = types2.NewDictionary()
	L1Collector.gpsVerifier = types2.NewDictionary()
	L1Collector.facts = types2.NewDictionary()
	L1Collector.Synced = false
	go L1Collector.updateLatestBlockOnChain()
}

func (l *l1Collector) Run() error {
	l.logger.Info("Service Started")

	l.latestBlockSynced = l.manager.GetLatestBlockSync()

	go l.handleEvents()

	// start the buffer updater
	for {
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
				l.logger.With("Error", err).Debug("Error processing pages hashes")
				time.Sleep(time.Second * 3)
				continue
			}

			stateDiff := parsePages(pages)
			stateDiff.NewRoot = fact.StateRoot
			stateDiff.BlockNumber = int64(fact.SequenceNumber)
			l.buffer <- stateDiff

			l.removeFactTree(fact)
			l.logger.With("BlockNumber", l.latestBlockSynced).Info("StateDiff collected")
			l.latestBlockSynced += 1
		}
	}
}

func (l *l1Collector) GetChannel() chan *types2.StateDiff {
	return l.buffer
}

func (l *l1Collector) Close(context.Context) {
	close(l.buffer)
}

func (l *l1Collector) LatestBlock() *feeder.StarknetBlock {
	return l.latestBlock
}

func (l *l1Collector) PendingBlock() *feeder.StarknetBlock {
	return l.pendingBlock
}

func (l *l1Collector) IsSynced() bool {
	return l.Synced
}

func (l *l1Collector) updateLatestBlockOnChain() {
	go l.updatePendingBlock()
	l.loadContractsAbi()
	number, err := l.l1client.BlockNumber(context.Background())
	if err != nil {
		l.logger.Error("Error subscribing to logs", "err", err)
		return
	}
	// build query for the latest block
	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetInt64(int64(number) - int64(10*windowSize)),
		// ToBlock:   new(big.Int).SetInt64(int64(number)),
		Addresses: []common.Address{l.starknetContractAddress},
		Topics:    [][]common.Hash{{crypto.Keccak256Hash([]byte(l.starknetABI.Events["LogStateUpdate"].Sig))}},
	}
	logs, err := l.l1client.FilterLogs(context.Background(), query)
	if err != nil {
		l.logger.Error("Error subscribing to logs", "err", err)
		return
	}
	for _, logFetched := range logs {
		l.updateBlockOnChain(logFetched.Data)
	}

	subLogs := make(chan types.Log)
	subscription, err := l.l1client.SubscribeFilterLogs(context.Background(), query, subLogs)
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
		txn, _, err := l.l1client.TransactionByHash(context.Background(), txHash)
		if err != nil {
			l.logger.With("Error", err, "Transaction Hash", v).
				Error("Couldn't retrieve transactions")
			return nil, err
		}

		// Parse Ethereum transaction calldata for Starknet transaction information
		data := txn.Data()[4:] // Remove the method signature hash
		inputs := make(map[string]interface{})
		err = memoryContract.Methods["registerContinuousMemoryPage"].Inputs.UnpackIntoMap(inputs, data)
		if err != nil {
			l.logger.With("Error", err).Info("Couldn't unpack into map")
			return nil, err
		}
		// Append calldata to pages
		pages = append(pages, inputs["values"].([]*big.Int))
	}
	return pages, nil
}

func (l *l1Collector) loadContractsAbi() {
	l.contractInfo = make(map[common.Address]types2.ContractInfo)

	contractAddresses, err := l.client.GetContractAddresses()
	if err != nil {
		l.logger.With("Error", err).Panic("Couldn't get ContractInfo Address from Feeder Gateway")
	}

	// Add Starknet contract
	err = loadContractInfo(contractAddresses.Starknet,
		abi.StarknetAbi,
		"LogStateTransitionFact", l.contractInfo)
	if err != nil {
		l.logger.With("Address", contractAddresses.Starknet).
			Panic("Couldn't load contract from disk ")
	}
	l.starknetContractAddress = common.HexToAddress(contractAddresses.Starknet)

	// Add Gps Statement Verifier contract
	gpsAddress := getGpsVerifierContractAddress(l.chainID)
	err = loadContractInfo(gpsAddress,
		abi.GpsVerifierAbi,
		"LogMemoryPagesHashes", l.contractInfo)
	if err != nil {
		l.logger.With("Address", gpsAddress).
			Panic("Couldn't load contract from disk ")
		return
	}
	l.gpsVerifierContractAddress = common.HexToAddress(gpsAddress)

	// Add Memory Page Fact Registry contract
	memoryPagesContractAddress := getMemoryPagesContractAddress(l.chainID)
	err = loadContractInfo(memoryPagesContractAddress,
		abi.MemoryPagesAbi,
		"LogMemoryPageFactContinuous", l.contractInfo)
	if err != nil {
		l.logger.With("Address", memoryPagesContractAddress).
			Panic("Couldn't load contract from disk ")
		return
	}
	l.memoryPagesContractAddress = common.HexToAddress(memoryPagesContractAddress)
}

func (l *l1Collector) handleEvents() {
	// Get the block in which we are going to start processing events
	// - 100 blocks back just in case.
	initialBlock := l.manager.GetBlockOfProcessedEvent(l.latestBlockSynced) - int64(windowSize)

	initialBlock = int64(math.Max(float64(initialBlock), float64(initialBlockForStarknetContract(l.chainID))))
	blockNumber, err := l.l1client.BlockNumber(context.Background())
	if err != nil {
		l.logger.Error("Error fetching latest block on Ethereum", "err", err)
		return
	}

	// Keep updated the blockNumber of the latest block on L1
	go func() {
		blockNumber, err = l.l1client.BlockNumber(context.Background())
		if err != nil {
			l.logger.Error("Error fetching latest block on Ethereum", "err", err)
		}
		time.Sleep(time.Second * 5)
	}()
	// Process blocks until latest
	for initialBlock < int64(blockNumber) {
		if len(l.buffer) > 8 {
			l.logger.Info("Buffer contains some elements, waiting to get more events")
			time.Sleep(time.Second)
			continue
		}
		err = l.processBatchOfEvents(initialBlock, int64(windowSize))
		if err != nil {
			l.logger.Error("Error processing batch of events", "err", err)
			return
		}
		initialBlock += int64(windowSize)
	}

	// If we are going to subscribe to the logs, that means we are currently synced with the latest block
	l.Synced = true

	// Subscribe for new blocks
	err = l.processSubscription(initialBlock)
	if err != nil {
		l.logger.With("Error", err).Error("Error subscribing to events")
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
	sub, err := l.l1client.SubscribeFilterLogs(context.Background(), query, hLog)
	if err != nil {
		l.logger.Info("Couldn't subscribe for incoming blocks")
		return err
	}
	for {
		select {
		case err := <-sub.Err():
			l.logger.With("Error", err).Debug("Error getting the latest logs")
		case vLog := <-hLog:
			event := map[string]interface{}{}
			err = l.contractInfo[vLog.Address].Contract.UnpackIntoMap(event, l.contractInfo[vLog.Address].EventName, vLog.Data)
			if err != nil {
				l.logger.With("Error", err).Debug("Couldn't get event from log")
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

	l.logger.With("From Block", initialBlock, "To Block", initialBlock+window).Info("Fetching logs....")
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(initialBlock),
		ToBlock:   big.NewInt(initialBlock + window),
		Addresses: addresses,
		Topics:    [][]common.Hash{topics},
	}

	starknetLogs, err := l.l1client.FilterLogs(context.Background(), query)
	if err != nil {
		l.logger.With("Error", err, "Initial block", initialBlock, "End block", initialBlock+window, "Addresses", addresses).
			Info("Couldn't get logs")
		return err
	}
	for _, vLog := range starknetLogs {
		event := map[string]interface{}{}

		err = l.contractInfo[vLog.Address].Contract.UnpackIntoMap(event, l.contractInfo[vLog.Address].EventName, vLog.Data)
		if err != nil {
			l.logger.With("Error", err).Info("Couldn't get LogStateTransitionFact from event")
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

		starknetLogs, err := l.l1client.FilterLogs(context.Background(), query)
		if err != nil {
			l.logger.With("Error", err, "Initial block", event.Block, "End block", event.Block+1).
				Info("Couldn't get logs")
		}
		fullFact, err := l.getFactInfo(starknetLogs, common.BytesToHash(b), event.TxnHash)
		if err != nil {
			l.logger.With("Error", err).Info("Couldn't get fact info")
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
			l.logger.With("Error", err).Info("Couldn't get state root or sequence number from LogStateUpdate event")
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
	l.logger.Panic("Couldn't find a block number that match in the logs for given fact")
	return nil, nil
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
		l.logger.With("Error", err).Info("Couldn't get state root or sequence number from LogStateUpdate event")
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
func parsePages(pages [][]*big.Int) *types2.StateDiff {
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
		storageDiff[address.String()] = kvs
	}

	return &types2.StateDiff{
		DeployedContracts: deployedContracts,
		StorageDiff:       storageDiff,
	}
}

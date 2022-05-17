// Package starknet contains all the functions related to Starknet State and Synchronization
// with Layer 2
package starknet

import (
	"context"
	"fmt"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/services"
	base "github.com/NethermindEth/juno/pkg/common"
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"strconv"
	"sync"
	"time"
)

// Synchronizer represents the base struct for Starknet Synchronization
type Synchronizer struct {
	ethereumClient         *ethclient.Client
	feederGatewayClient    *feeder.Client
	database               db.Databaser
	transactioner          db.Transactioner
	MemoryPageHash         base.Dictionary
	GpsVerifier            base.Dictionary
	latestMemoryPageBlock  int64
	latestGpsVerifierBlock int64
	facts                  []Fact
	lock                   sync.RWMutex
}

// NewSynchronizer creates a new Synchronizer
func NewSynchronizer(txnDb db.Databaser) *Synchronizer {
	client, err := ethclient.Dial(config.Runtime.Ethereum.Node)
	if err != nil {
		log.Default.With("Error", err).Fatal("Unable to connect to Ethereum Client")
	}
	fClient := feeder.NewClient(config.Runtime.Starknet.FeederGateway, "/feeder_gateway", nil)
	return &Synchronizer{
		ethereumClient:      client,
		feederGatewayClient: fClient,
		database:            txnDb,
		MemoryPageHash:      base.Dictionary{},
		GpsVerifier:         base.Dictionary{},
		facts:               make([]Fact, 0),
		transactioner:       db.NewTransactionDb(txnDb.GetEnv()),
	}
}

// UpdateState keeps updated the Starknet State in a process
func (s *Synchronizer) UpdateState() error {
	log.Default.Info("Starting to update state")
	//if config.Runtime.Starknet.FastSync {
	//	s.apiSync()
	//	return nil
	//}

	err := s.l1Sync()
	if err != nil {
		return err
	}
	return nil
}

func (s *Synchronizer) loadEvents(contracts map[common.Address]ContractInfo, eventChan chan eventInfo) error {
	addresses := make([]common.Address, 0)

	topics := make([]common.Hash, 0)
	for k, v := range contracts {
		addresses = append(addresses, k)
		topics = append(topics, crypto.Keccak256Hash([]byte(v.contract.Events[v.eventName].Sig)))
	}
	latestBlockNumber, err := s.ethereumClient.BlockNumber(context.Background())
	if err != nil {
		log.Default.With("Error", err).Error("Couldn't get the latest block")
		return err
	}

	initialBlock := initialBlockForStarknetContract(s.ethereumClient)
	increment := uint64(MaxChunk)
	i := uint64(initialBlock)
	for i < latestBlockNumber {
		log.Default.With("From Block", i, "To Block", i+increment).Info("Fetching logs....")
		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(i)),
			ToBlock:   big.NewInt(int64(i + increment)),
			Addresses: addresses,
			Topics:    [][]common.Hash{topics},
		}

		starknetLogs, err := s.ethereumClient.FilterLogs(context.Background(), query)
		if err != nil {
			log.Default.With("Error", err, "Initial block", i, "End block", i+increment, "Addresses", addresses).
				Info("Couldn't get logs")
			break
		}
		log.Default.With("Count", len(starknetLogs)).Info("Logs fetched")
		for _, vLog := range starknetLogs {
			log.Default.With("Log Fetched", contracts[vLog.Address].eventName, "BlockHash", vLog.BlockHash.Hex(), "BlockNumber", vLog.BlockNumber,
				"TxHash", vLog.TxHash.Hex()).Info("Event Fetched")
			event := map[string]interface{}{}

			err = contracts[vLog.Address].contract.UnpackIntoMap(event, contracts[vLog.Address].eventName, vLog.Data)
			if err != nil {
				log.Default.With("Error", err).Info("Couldn't get LogStateTransitionFact from event")
				continue
			}
			eventChan <- eventInfo{
				block:           vLog.BlockNumber,
				event:           event,
				address:         contracts[vLog.Address].address,
				transactionHash: vLog.TxHash,
			}
		}
		i += increment
	}
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(latestBlockNumber)),
		Addresses: addresses,
	}
	hLog := make(chan types.Log)
	sub, err := s.ethereumClient.SubscribeFilterLogs(context.Background(), query, hLog)
	if err != nil {
		log.Default.Info("Couldn't subscribe for incoming blocks")
		return err
	}
	for {
		select {
		case err := <-sub.Err():
			log.Default.With("Error", err).Info("Error getting the latest logs")
		case vLog := <-hLog:
			log.Default.With("Log Fetched", contracts[vLog.Address].eventName, "BlockHash", vLog.BlockHash.Hex(),
				"BlockNumber", vLog.BlockNumber, "TxHash", vLog.TxHash.Hex()).
				Info("Event Fetched")
			event := map[string]interface{}{}
			err = contracts[vLog.Address].contract.UnpackIntoMap(event, contracts[vLog.Address].eventName, vLog.Data)
			if err != nil {
				log.Default.With("Error", err).Info("Couldn't get event from log")
				continue
			}
			eventChan <- eventInfo{
				block:           vLog.BlockNumber,
				event:           event,
				address:         contracts[vLog.Address].address,
				transactionHash: vLog.TxHash,
			}
		}
	}
}

func (s *Synchronizer) latestBlockOnChain() (uint64, error) {
	number, err := s.ethereumClient.BlockNumber(context.Background())
	if err != nil {
		log.Default.With("Error", err).Error("Couldn't get the latest block")
		return 0, err
	}
	return number, nil
}

func (s *Synchronizer) l1Sync() error {
	log.Default.Info("Starting to update state")

	contractAddresses, err := s.feederGatewayClient.GetContractAddresses()
	if err != nil {
		log.Default.With("Error", err).Panic("Couldn't get ContractInfo Address from Feeder Gateway")
		return err
	}
	event := make(chan eventInfo)
	contracts := make(map[common.Address]ContractInfo)

	// Add Starknet contract
	err = loadContractInfo(contractAddresses.Starknet,
		config.Runtime.Starknet.ContractAbiPathConfig.StarknetAbiPath,
		"LogStateTransitionFact", contracts)
	if err != nil {
		log.Default.With("Address", contractAddresses.Starknet,
			"Value Path", config.Runtime.Starknet.ContractAbiPathConfig.StarknetAbiPath).
			Panic("Couldn't load contract from disk ")
		return err
	}

	// Add Gps Statement Verifier contract
	gpsAddress := getGpsVerifierContractAddress(s.ethereumClient)
	err = loadContractInfo(gpsAddress,
		config.Runtime.Starknet.ContractAbiPathConfig.GpsVerifierAbiPath,
		"LogMemoryPagesHashes", contracts)
	if err != nil {
		log.Default.With("Address", gpsAddress,
			"Value Path", config.Runtime.Starknet.ContractAbiPathConfig.GpsVerifierAbiPath).
			Panic("Couldn't load contract from disk ")
		return err
	}
	// Add Memory Page Fact Registry contract
	memoryPagesContractAddress := getMemoryPagesContractAddress(s.ethereumClient)
	err = loadContractInfo(memoryPagesContractAddress,
		config.Runtime.Starknet.ContractAbiPathConfig.MemoryPageAbiPath,
		"LogMemoryPageFactContinuous", contracts)
	if err != nil {
		log.Default.With("Address", gpsAddress,
			"Value Path", config.Runtime.Starknet.ContractAbiPathConfig.GpsVerifierAbiPath).
			Panic("Couldn't load contract from disk ")
		return err
	}

	go func() {

		err = s.loadEvents(contracts, event)
		if err != nil {
			log.Default.With("Error", err).Info("Couldn't get events")
			close(event)
		}
	}()

	// Handle frequently if there is any fact that comes from L1 to handle
	go func() {
		ticker := time.NewTicker(time.Second * 5)

		for {
			select {
			case <-ticker.C:
				if len(s.facts) == 0 {
					continue
				}
				if s.GpsVerifier.Exist(s.facts[0].value) {
					s.lock.Lock()
					// If already exist the information related to the fact,
					// fetch the memory pages and updated the State
					s.processMemoryPages(s.facts[0].value, s.facts[0].stateRoot, s.facts[0].blockNumber)
					s.facts = s.facts[1:]
					s.lock.Unlock()
				}
			}
		}
	}()

	for {
		select {
		case l, ok := <-event:
			if !ok {
				return fmt.Errorf("couldn't read event from logs")
			}
			// Process GpsStatementVerifier contract
			factHash, ok := l.event["factHash"]
			pagesHashes, ok1 := l.event["pagesHashes"]

			if ok && ok1 {
				b := make([]byte, 0)
				for _, v := range factHash.([32]byte) {
					b = append(b, v)
				}
				s.GpsVerifier.Add(common.BytesToHash(b).Hex(), pagesHashes.([][32]byte))
			}
			// Process MemoryPageFactRegistry contract
			if memoryHash, ok := l.event["memoryHash"]; ok {
				key := common.BytesToHash(memoryHash.(*big.Int).Bytes()).Hex()
				value := l.transactionHash
				s.MemoryPageHash.Add(key, value)
			}
			// Process Starknet logs
			if fact, ok := l.event["stateTransitionFact"]; ok {

				b := make([]byte, 0)
				for _, v := range fact.([32]byte) {
					b = append(b, v)
				}
				abi, _ := loadAbiOfContract(config.Runtime.Starknet.ContractAbiPathConfig.StarknetAbiPath)
				starknetAddress := common.HexToAddress(contractAddresses.Starknet)
				fullFact := getFactInfo(s.ethereumClient, ContractInfo{contract: abi, eventName: "LogStateUpdate",
					address: starknetAddress}, l.block, common.BytesToHash(b).Hex())

				s.lock.Lock()
				s.facts = append(s.facts, fullFact)
				s.lock.Unlock()

			}

		}
	}
}

func getFactInfo(client *ethclient.Client, info ContractInfo, block uint64, fact string) Fact {
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(block)),
		ToBlock:   big.NewInt(int64(block)),
		Addresses: []common.Address{info.address},
		Topics:    [][]common.Hash{{crypto.Keccak256Hash([]byte(info.contract.Events["LogStateUpdate"].Sig))}},
	}

	starknetLogs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		log.Default.With("Error", err, "Initial block", block, "End block", block+1).
			Info("Couldn't get logs")
	}
	if len(starknetLogs) != 1 {

	}
	vLog := starknetLogs[0]
	log.Default.With("Log Fetched", "LogStateUpdate", "BlockHash", vLog.BlockHash.Hex(),
		"BlockNumber", vLog.BlockNumber, "TxHash", vLog.TxHash.Hex()).Info("Event Fetched")
	event := map[string]interface{}{}

	err = info.contract.UnpackIntoMap(event, "LogStateUpdate", vLog.Data)
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't get LogStateTransitionFact from event")
		return Fact{}
	}
	return Fact{
		stateRoot:   common.BigToHash(event["globalRoot"].(*big.Int)).String(),
		blockNumber: strconv.FormatInt(event["blockNumber"].(*big.Int).Int64(), 10),
		value:       fact,
	}
}

// Close closes the client for the Layer 1 Ethereum node
func (s *Synchronizer) Close(ctx context.Context) {
	// notest
	log.Default.Info("Closing Layer 1 Synchronizer")
	s.ethereumClient.Close()
	//(*s.database).Close()
}

func (s *Synchronizer) apiSync() {
	latestBlockQueried, err := latestBlockQueried(s.database)
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't get latest Block queried")
		return
	}
	blockIterator := int(latestBlockQueried)
	lastBlockHash := ""
	for {
		newValueForIterator, newBlockHash := s.updateStateForOneBlock(blockIterator, lastBlockHash)
		if newBlockHash == lastBlockHash {
			break
		}
		lastBlockHash = newBlockHash
		blockIterator = newValueForIterator
	}

	ticker := time.NewTicker(time.Minute * 2)
	for {
		select {
		case <-ticker.C:
			newValueForIterator, newBlockHash := s.updateStateForOneBlock(blockIterator, lastBlockHash)
			if newBlockHash == lastBlockHash {
				break
			}
			lastBlockHash = newBlockHash
			blockIterator = newValueForIterator
		}
	}
}

func (s *Synchronizer) updateStateForOneBlock(blockIterator int, lastBlockHash string) (int, string) {
	log.Default.With("Number", blockIterator).Info("Updating StarkNet State")
	update, err := s.feederGatewayClient.GetStateUpdate("", strconv.Itoa(blockIterator))
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't get state update")
		return blockIterator, lastBlockHash
	}
	if lastBlockHash == update.BlockHash {
		return blockIterator, lastBlockHash
	}
	log.Default.With("Block Hash", update.BlockHash, "New Root", update.NewRoot, "Old Root", update.OldRoot).
		Info("Updating state")

	upd := stateUpdateResponseToStateDiff(update)

	err = s.updateState(upd, update.NewRoot, strconv.Itoa(blockIterator))
	if err != nil {
		log.Default.With("Error", err).Panic("Couldn't update state")
	}
	log.Default.With("Block Number", blockIterator).Info("State updated")
	err = updateLatestBlockQueried(s.database, int64(blockIterator))
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't save latest block queried")
	}
	return blockIterator + 1, update.BlockHash
}

func (s *Synchronizer) updateState(update StateDiff, stateRoot, blockNumber string) error {
	log.Default.With("Block Number", blockNumber).Info("Processing block")

	txn := s.transactioner.Begin()

	stateTrie := newTrie(txn, "state_trie_")

	for _, deployedContract := range update.DeployedContracts {
		contractHash, ok := new(big.Int).SetString(remove0x(deployedContract.ContractHash), 16)
		if !ok {
			log.Default.Panic("Couldn't get contract hash")
		}
		storeContractHash(deployedContract.Address, contractHash)
		storageTrie := newTrie(txn, remove0x(deployedContract.Address))
		storageRoot := storageTrie.Commitment()
		address, ok := new(big.Int).SetString(remove0x(deployedContract.Address), 16)
		if !ok {
			log.Default.With("Address", deployedContract.Address).
				Panic("Couldn't convert Address to Big.Int ")
		}
		contractStateValue := contractState(contractHash, storageRoot)
		stateTrie.Put(address, contractStateValue)
	}

	for k, v := range update.StorageDiffs {
		storageTrie := newTrie(txn, remove0x(k))
		for _, storageSlots := range v {
			key, ok := new(big.Int).SetString(remove0x(storageSlots.Key), 16)
			if !ok {
				log.Default.With("Storage Slot Key", storageSlots.Key).
					Panic("Couldn't get the ")
			}
			val, ok := new(big.Int).SetString(remove0x(storageSlots.Value), 16)
			if !ok {
				log.Default.With("Storage Slot Value", storageSlots.Value).
					Panic("Couldn't get the contract Hash")
			}
			storageTrie.Put(key, val)
		}
		storageRoot := storageTrie.Commitment()

		address, ok := new(big.Int).SetString(remove0x(k), 16)
		if !ok {
			log.Default.With("Address", k).
				Panic("Couldn't convert Address to Big.Int ")
		}
		contractStateValue := contractState(loadContractHash(k), storageRoot)

		stateTrie.Put(address, contractStateValue)
	}

	stateCommitment := remove0x(stateTrie.Commitment().Text(16))

	if stateRoot != "" && stateCommitment != remove0x(stateRoot) {
		log.Default.With("State Commitment", stateCommitment, "State Root from API", remove0x(stateRoot)).
			Panic("stateRoot not equal to the one provided")
	}
	err := txn.Commit()
	if err != nil {
		log.Default.Panic("Couldn't commit to the database")
		return err
	}

	log.Default.With("State Root", stateCommitment).
		Info("Got State commitment")

	//s.updateAbiAndCode(update, blockHash, blockNumber)
	return nil
}

func (s *Synchronizer) processMemoryPages(fact, stateRoot, blockNumber string) {
	pages := make([][]*big.Int, 0)

	// Get memory pages hashes using fact
	var memoryPages [][32]byte
	memoryPages = (s.GpsVerifier.Get(fact)).([][32]byte)
	memoryContract, err := loadAbiOfContract(config.Runtime.Starknet.ContractAbiPathConfig.MemoryPageAbiPath)
	if err != nil {
		return
	}

	// iterate over each memory page
	for _, v := range memoryPages {
		h := make([]byte, 0)

		for _, s := range v {
			h = append(h, s)
		}
		// Get transactionsHash based on the memory page
		hash := common.BytesToHash(h)
		transactionHash := s.MemoryPageHash.Get(hash.Hex())
		log.Default.With("Hash", hash.Hex()).Info("Getting transaction...")
		txn, _, err := s.ethereumClient.TransactionByHash(context.Background(), transactionHash.(common.Hash))
		if err != nil {
			log.Default.With("Error", err, "Transaction Hash", v).
				Error("Couldn't retrieve transactions")
			return
		}
		method := memoryContract.Methods["registerContinuousMemoryPage"]

		data := txn.Data()
		if len(txn.Data()) < 5 {
			log.Default.Error("memory page transaction input has incomplete signature")
			continue
		}
		inputs := make(map[string]interface{})

		// unpack method inputs
		err = method.Inputs.UnpackIntoMap(inputs, data[4:])
		if err != nil {
			log.Default.With("Error", err).Info("Couldn't unpack into map")
			return
		}
		t, _ := inputs["values"]
		// Get the inputs of the transaction from Layer 1
		// Append to the memory pages
		pages = append(pages, t.([]*big.Int))
	}
	// pages should contain all txn information
	s.parsePages(pages, stateRoot, blockNumber)
}

func (s *Synchronizer) updateAbiAndCode(update StateDiff, blockHash, blockNumber string) {
	for _, v := range update.DeployedContracts {
		code, err := s.feederGatewayClient.GetCode(v.Address, blockHash, blockNumber)
		if err != nil {
			return
		}
		log.Default.
			With("ContractInfo Address", v.Address, "Block Hash", blockHash, "Block Number", blockNumber).
			Info("Fetched code and ABI")
		// Save the ABI
		abiService := services.GetABIService()
		abiService.StoreABI(remove0x(v.Address), *code.Abi)
		// Save the contract code
		stateService := services.GetStateService()
		stateService.StoreCode(remove0x(v.Address), code.Bytecode)
	}
}

func (s *Synchronizer) updateBlocksAndTransactions(update feeder.StateUpdateResponse) {
	block, err := s.feederGatewayClient.GetBlock(update.BlockHash, "")
	if err != nil {
		return
	}
	log.Default.With("Block Hash", update.BlockHash).
		Info("Got block")
	// TODO: Store block, where to store it? How to store it?

	for _, bTxn := range block.Transactions {
		transactionInfo, err := s.feederGatewayClient.GetTransaction(bTxn.TransactionHash, "")
		if err != nil {
			return
		}
		log.Default.With("Transaction Hash", transactionInfo.Transaction.TransactionHash).
			Info("Got transactions of block")
		// TODO: Store transactions, where to store it? How to store it?

	}

}

// parsePages parse the pages returned from the interaction with Layer 1
func (s *Synchronizer) parsePages(pages [][]*big.Int, stateRoot, blockNumber string) {
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
	deployedContracts := make([]DeployedContract, 0)

	// Get the info of the deployed contracts
	deployedContractsData := pagesFlatter[:deployedContractsInfoLen]

	// Iterate while contains contract data to be processed
	for len(deployedContractsData) > 0 {
		// Parse the Address of the contract
		address := common.Bytes2Hex(deployedContractsData[0].Bytes())
		deployedContractsData = deployedContractsData[1:]

		// Parse the ContractInfo Hash
		contractHash := common.Bytes2Hex(deployedContractsData[0].Bytes())
		deployedContractsData = deployedContractsData[1:]

		// Parse the number of Arguments the constructor contains
		constructorArgumentsLen := deployedContractsData[0].Int64()
		deployedContractsData = deployedContractsData[1:]

		// Parse constructor arguments
		constructorArguments := make([]*big.Int, 0)
		for i := int64(0); i < constructorArgumentsLen; i++ {
			constructorArguments = append(constructorArguments, deployedContractsData[0])
			deployedContractsData = deployedContractsData[1:]
		}

		// Store deployed ContractInfo information
		deployedContracts = append(deployedContracts, DeployedContract{
			Address:             address,
			ContractHash:        contractHash,
			ConstructorCallData: constructorArguments,
		})
	}
	pagesFlatter = pagesFlatter[deployedContractsInfoLen:]

	// Parse the number of contracts updates
	numContractsUpdate := pagesFlatter[0].Int64()
	pagesFlatter = pagesFlatter[1:]

	storageDiffs := make(map[string][]KV, 0)

	// Iterate over all the contracts that had been updated and collect the needed information
	for i := int64(0); i < numContractsUpdate; i++ {
		// Parse the Address of the contract
		address := common.Bytes2Hex(pagesFlatter[0].Bytes())
		pagesFlatter = pagesFlatter[1:]

		// Parse the number storage updates
		numStorageUpdates := pagesFlatter[0].Int64()
		pagesFlatter = pagesFlatter[1:]

		kvs := make([]KV, 0)
		for k := int64(0); k < numStorageUpdates; k++ {
			kvs = append(kvs, KV{
				Key:   common.Bytes2Hex(pagesFlatter[0].Bytes()),
				Value: common.Bytes2Hex(pagesFlatter[1].Bytes()),
			})
			pagesFlatter = pagesFlatter[2:]
		}
		storageDiffs[address] = kvs
	}

	state := StateDiff{
		DeployedContracts: deployedContracts,
		StorageDiffs:      storageDiffs,
	}

	err := s.updateState(state, stateRoot, blockNumber)
	if err != nil {
		log.Default.With("Error", err).Panic("Couldn't update state")
	}
	log.Default.With("Block Number", blockNumber).Info("State updated")
	bNumber, _ := strconv.Atoi(blockNumber)
	err = updateLatestBlockQueried(s.database, int64(bNumber))
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't save latest block queried")
	}
}

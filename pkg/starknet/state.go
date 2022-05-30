// Package starknet contains all the functions related to Starknet State and Synchronization
// with Layer 2
package starknet

import (
	"context"
	"fmt"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/starknet/abi"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"strconv"
	"time"
)

// Synchronizer represents the base struct for Starknet Synchronization
type Synchronizer struct {
	ethereumClient      *ethclient.Client
	feederGatewayClient *feeder.Client
	database            db.Databaser
	transactioner       db.Transactioner
	memoryPageHash      *starknetTypes.Dictionary
	gpsVerifier         *starknetTypes.Dictionary
	facts               *starknetTypes.Dictionary
	chainID             int64
}

// NewSynchronizer creates a new Synchronizer
func NewSynchronizer(txnDb db.Databaser, client *ethclient.Client, fClient *feeder.Client) *Synchronizer {
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Default.Panic("Unable to retrieve chain ID from Ethereum Node")
	}
	return &Synchronizer{
		ethereumClient:      client,
		feederGatewayClient: fClient,
		database:            txnDb,
		memoryPageHash:      starknetTypes.NewDictionary(txnDb, "memory_pages"),
		gpsVerifier:         starknetTypes.NewDictionary(txnDb, "gps_verifier"),
		facts:               starknetTypes.NewDictionary(txnDb, "facts"),
		chainID:             chainID.Int64(),
		transactioner:       db.NewTransactionDb(txnDb.GetEnv()),
	}
}

// UpdateState keeps updated the Starknet State in a process
func (s *Synchronizer) UpdateState() error {
	log.Default.Info("Starting to update state")
	if config.Runtime.Starknet.ApiSync {
		return s.apiSync()
	}
	return s.l1Sync()
}

func (s *Synchronizer) loadEvents(contracts map[common.Address]starknetTypes.ContractInfo, eventChan chan starknetTypes.EventInfo) error {
	addresses := make([]common.Address, 0)

	topics := make([]common.Hash, 0)
	for k, v := range contracts {
		addresses = append(addresses, k)
		topics = append(topics, crypto.Keccak256Hash([]byte(v.Contract.Events[v.EventName].Sig)))
	}
	latestBlockNumber, err := s.ethereumClient.BlockNumber(context.Background())
	if err != nil {
		log.Default.With("Error", err).Error("Couldn't get the latest block")
		return err
	}

	initialBlock := initialBlockForStarknetContract(s.chainID)
	increment := uint64(starknetTypes.MaxChunk)
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
			log.Default.With("Log Fetched", contracts[vLog.Address].EventName, "BlockHash", vLog.BlockHash.Hex(), "BlockNumber", vLog.BlockNumber,
				"TxHash", vLog.TxHash.Hex()).Info("Event Fetched")
			event := map[string]interface{}{}

			err = contracts[vLog.Address].Contract.UnpackIntoMap(event, contracts[vLog.Address].EventName, vLog.Data)
			if err != nil {
				log.Default.With("Error", err).Info("Couldn't get LogStateTransitionFact from event")
				continue
			}
			eventChan <- starknetTypes.EventInfo{
				Block:           vLog.BlockNumber,
				Event:           event,
				Address:         contracts[vLog.Address].Address,
				TransactionHash: vLog.TxHash,
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
			log.Default.With("Log Fetched", contracts[vLog.Address].EventName, "BlockHash", vLog.BlockHash.Hex(),
				"BlockNumber", vLog.BlockNumber, "TxHash", vLog.TxHash.Hex()).
				Info("Event Fetched")
			event := map[string]interface{}{}
			err = contracts[vLog.Address].Contract.UnpackIntoMap(event, contracts[vLog.Address].EventName, vLog.Data)
			if err != nil {
				log.Default.With("Error", err).Info("Couldn't get event from log")
				continue
			}
			eventChan <- starknetTypes.EventInfo{
				Block:           vLog.BlockNumber,
				Event:           event,
				Address:         contracts[vLog.Address].Address,
				TransactionHash: vLog.TxHash,
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
	event := make(chan starknetTypes.EventInfo)
	contracts := make(map[common.Address]starknetTypes.ContractInfo)

	// Add Starknet contract
	err = loadContractInfo(contractAddresses.Starknet,
		abi.StarknetAbi,
		"LogStateTransitionFact", contracts)
	if err != nil {
		log.Default.With("Address", contractAddresses.Starknet).
			Panic("Couldn't load contract from disk ")
		return err
	}

	// Add Gps Statement Verifier contract
	gpsAddress := getGpsVerifierContractAddress(s.chainID)
	err = loadContractInfo(gpsAddress,
		abi.GpsVerifierAbi,
		"LogMemoryPagesHashes", contracts)
	if err != nil {
		log.Default.With("Address", gpsAddress).
			Panic("Couldn't load contract from disk ")
		return err
	}
	// Add Memory Page Fact Registry contract
	memoryPagesContractAddress := getMemoryPagesContractAddress(s.chainID)
	err = loadContractInfo(memoryPagesContractAddress,
		abi.MemoryPagesAbi,
		"LogMemoryPageFactContinuous", contracts)
	if err != nil {
		log.Default.With("Address", gpsAddress).
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
		for range ticker.C {
			factSynced, err := getNumericValueFromDB(s.database, starknetTypes.LatestFactSynced)
			if err != nil {
				log.Default.With("Error", err).
					Info("Unable to get the Value of the latest fact synced")
				continue
			}
			if !s.facts.Exist(strconv.FormatInt(factSynced, 10)) {
				continue
			}
			f := starknetTypes.Fact{}
			fact, _ := s.facts.Get(strconv.FormatInt(factSynced, 10), f)

			if s.gpsVerifier.Exist(fact.(starknetTypes.Fact).Value) {
				// If already exist the information related to the fact,
				// fetch the memory pages and updated the State
				s.processMemoryPages(fact.(starknetTypes.Fact).Value, fact.(starknetTypes.Fact).StateRoot, fact.(starknetTypes.Fact).BlockNumber)
				s.facts.Remove(strconv.FormatInt(factSynced, 10))
				err = updateNumericValueFromDB(s.database, starknetTypes.LatestFactSynced, factSynced)
				if err != nil {
					return
				}
			}
		}
	}()

	for l := range event {
		// Process GpsStatementVerifier contract
		factHash, ok := l.Event["factHash"]
		pagesHashes, ok1 := l.Event["pagesHashes"]

		if ok && ok1 {
			b := make([]byte, 0)
			for _, v := range factHash.([32]byte) {
				b = append(b, v)
			}
			value := starknetTypes.PagesHash{Bytes: pagesHashes.([][32]byte)}
			s.gpsVerifier.Add(common.BytesToHash(b).Hex(), value)
		}
		// Process MemoryPageFactRegistry contract
		if memoryHash, ok := l.Event["memoryHash"]; ok {
			key := common.BytesToHash(memoryHash.(*big.Int).Bytes()).Hex()
			value := starknetTypes.TransactionHash{Hash: l.TransactionHash}
			s.memoryPageHash.Add(key, value)
		}
		// Process Starknet logs
		if fact, ok := l.Event["stateTransitionFact"]; ok {

			b := make([]byte, 0)
			for _, v := range fact.([32]byte) {
				b = append(b, v)
			}
			abiOfContract, _ := loadAbiOfContract(abi.StarknetAbi)
			starknetAddress := common.HexToAddress(contractAddresses.Starknet)

			factSaved, err := getNumericValueFromDB(s.database, starknetTypes.LatestFactSaved)
			if err != nil {
				log.Default.With("Error", err).
					Info("Unable to get the Value of the latest fact synced")
				return err
			}

			fullFact := getFactInfo(s.ethereumClient, starknetTypes.ContractInfo{Contract: abiOfContract, EventName: "LogStateUpdate",
				Address: starknetAddress}, l.Block, common.BytesToHash(b).Hex(), factSaved)

			// Safe Fact for block x
			s.facts.Add(strconv.FormatInt(factSaved, 10), fullFact)

			err = updateNumericValueFromDB(s.database, starknetTypes.LatestFactSaved, factSaved)
			if err != nil {
				log.Default.With("Error", err).
					Info("Unable to set the Value of the latest block synced")
				return err
			}
		}
	}
	return fmt.Errorf("couldn't read event from logs")
}

func getFactInfo(client *ethclient.Client, info starknetTypes.ContractInfo, block uint64, fact string, latestBlockSynced int64) starknetTypes.Fact {
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(block)),
		ToBlock:   big.NewInt(int64(block)),
		Addresses: []common.Address{info.Address},
		Topics:    [][]common.Hash{{crypto.Keccak256Hash([]byte(info.Contract.Events["LogStateUpdate"].Sig))}},
	}

	starknetLogs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		log.Default.With("Error", err, "Initial block", block, "End block", block+1).
			Info("Couldn't get logs")
	}
	for _, vLog := range starknetLogs {
		log.Default.With("Log Fetched", "LogStateUpdate", "BlockHash", vLog.BlockHash.Hex(),
			"BlockNumber", vLog.BlockNumber, "TxHash", vLog.TxHash.Hex()).Info("Event Fetched")
		event := map[string]interface{}{}

		err = info.Contract.UnpackIntoMap(event, "LogStateUpdate", vLog.Data)
		if err != nil {
			log.Default.With("Error", err).Info("Couldn't get LogStateTransitionFact from event")
			return starknetTypes.Fact{}
		}
		factVal := starknetTypes.Fact{
			StateRoot:   common.BigToHash(event["globalRoot"].(*big.Int)).String(),
			BlockNumber: strconv.FormatInt(event["blockNumber"].(*big.Int).Int64(), 10),
			Value:       fact,
		}
		if factVal.BlockNumber == strconv.FormatInt(latestBlockSynced, 10) {
			return factVal
		}

	}
	log.Default.Panic("Couldn't find a block number that match in the logs for given fact")
	return starknetTypes.Fact{}
}

// Close closes the client for the Layer 1 Ethereum node
func (s *Synchronizer) Close(ctx context.Context) {
	// notest
	log.Default.Info("Closing Layer 1 Synchronizer")
	s.ethereumClient.Close()
	//(*s.database).Close()
}

func (s *Synchronizer) apiSync() error {
	latestBlockQueried, err := getNumericValueFromDB(s.database, starknetTypes.LatestBlockSynced)
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't get latest Block queried")
		return err
	}
	blockIterator := int(latestBlockQueried)
	lastBlockHash := ""
	for {
		newValueForIterator, newBlockHash := s.updateStateForOneBlock(blockIterator, lastBlockHash)
		if newBlockHash == lastBlockHash {
			// Assume we are completely synced or an error has occurred
			time.Sleep(time.Minute * 2)
		}
		blockIterator, lastBlockHash = newValueForIterator, newBlockHash
	}
}

func (s *Synchronizer) updateStateForOneBlock(blockIterator int, lastBlockHash string) (int, string) {
	log.Default.With("Number", blockIterator).Info("Updating StarkNet State")
	update, err := s.feederGatewayClient.GetStateUpdate("", strconv.Itoa(blockIterator))
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't get state update")
		return blockIterator, lastBlockHash
	}
	if lastBlockHash == update.BlockHash || update.BlockHash == "" || update.NewRoot == "" {
		log.Default.With("Block Number", blockIterator).Info("Block is pending ...")
		return blockIterator, lastBlockHash
	}
	log.Default.With("Block Hash", update.BlockHash, "New Root", update.NewRoot, "Old Root", update.OldRoot).
		Info("Updating state")

	upd := stateUpdateResponseToStateDiff(update)

	txn := s.transactioner.Begin()
	hashService := services.GetContractHashService()
	if hashService == nil {
		log.Default.Panic("Contract hash service is unavailable")
	}
	_, err = updateState(txn, hashService, upd, update.NewRoot, strconv.Itoa(blockIterator))
	if err != nil {
		log.Default.With("Error", err).Panic("Couldn't update state")
	} else {
		err := txn.Commit()
		if err != nil {
			log.Default.Panic("Couldn't commit to the database")
		}
	}

	s.updateAbiAndCode(upd, lastBlockHash, string(rune(blockIterator)))

	log.Default.With("Block Number", blockIterator).Info("State updated")
	err = updateNumericValueFromDB(s.database, starknetTypes.LatestBlockSynced, int64(blockIterator))
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't save latest block queried")
	}
	return blockIterator + 1, update.BlockHash
}

func updateState(
	txn db.Transaction,
	hashService *services.ContractHashService,
	update starknetTypes.StateDiff,
	stateRoot, blockNumber string,
) (string, error) {
	log.Default.With("Block Number", blockNumber).Info("Processing block")

	stateTrie := newTrie(txn, "state_trie_")

	log.Default.With("Block Number", blockNumber).Info("Processing deployed contracts")
	for _, deployedContract := range update.DeployedContracts {
		contractHash, ok := new(big.Int).SetString(remove0x(deployedContract.ContractHash), 16)
		if !ok {
			log.Default.Panic("Couldn't get contract hash")
		}
		hashService.StoreContractHash(remove0x(deployedContract.Address), contractHash)
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

	log.Default.With("Block Number", blockNumber).Info("Processing storage diffs")
	for k, v := range update.StorageDiffs {
		formattedAddress := remove0x(k)
		storageTrie := newTrie(txn, formattedAddress)
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

		address, ok := new(big.Int).SetString(formattedAddress, 16)
		if !ok {
			log.Default.With("Address", formattedAddress).
				Panic("Couldn't convert Address to Big.Int ")
		}
		contractHash := hashService.GetContractHash(formattedAddress)
		contractStateValue := contractState(contractHash, storageRoot)

		stateTrie.Put(address, contractStateValue)
	}

	stateCommitment := remove0x(stateTrie.Commitment().Text(16))

	if stateRoot != "" && stateCommitment != remove0x(stateRoot) {
		log.Default.With("State Commitment", stateCommitment, "State Root from API", remove0x(stateRoot)).
			Panic("stateRoot not equal to the one provided")
	}
	log.Default.With("State Root", stateCommitment).
		Info("Got State commitment")

	return stateCommitment, nil
}

func (s *Synchronizer) processMemoryPages(fact, stateRoot, blockNumber string) {
	pages := make([][]*big.Int, 0)

	// Get memory pages hashes using fact
	valInterface := starknetTypes.PagesHash{}
	memoryPages, err := s.gpsVerifier.Get(fact, valInterface)
	if err != nil {
		return
	}
	if err != nil {
		return
	}
	memoryContract, err := loadAbiOfContract(abi.MemoryPagesAbi)
	if err != nil {
		return
	}

	// iterate over each memory page
	for _, v := range memoryPages.(starknetTypes.PagesHash).Bytes {
		h := make([]byte, 0)

		for _, s := range v {
			h = append(h, s)
		}
		// Get transactionsHash based on the memory page
		hash := common.BytesToHash(h)
		valInter := starknetTypes.TransactionHash{}
		transactionHash, err := s.memoryPageHash.Get(hash.Hex(), &valInter)
		if err != nil {
			return
		}
		log.Default.With("Hash", hash.Hex()).Info("Getting transaction...")
		txn, _, err := s.ethereumClient.TransactionByHash(context.Background(), transactionHash.(starknetTypes.TransactionHash).Hash)
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
		t := inputs["values"]
		// Get the inputs of the transaction from Layer 1
		// Append to the memory pages
		pages = append(pages, t.([]*big.Int))
	}
	// pages should contain all txn information
	state := parsePages(pages)

	txn := s.transactioner.Begin()
	hashService := services.GetContractHashService()
	if hashService == nil {
		log.Default.Panic("Contract hash service is unavailable")
	}
	_, err = updateState(txn, services.GetContractHashService(), *state, stateRoot, blockNumber)
	if err != nil {
		log.Default.With("Error", err).Panic("Couldn't update state")
	} else {
		err := txn.Commit()
		if err != nil {
			log.Default.Panic("Couldn't commit to the database")
		}
	}
	log.Default.With("Block Number", blockNumber).Info("State updated")
	bNumber, _ := strconv.Atoi(blockNumber)
	err = updateNumericValueFromDB(s.database, starknetTypes.LatestBlockSynced, int64(bNumber))
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't save latest block queried")
	}
}

func (s *Synchronizer) updateAbiAndCode(update starknetTypes.StateDiff, blockHash, blockNumber string) {
	for _, v := range update.DeployedContracts {
		code, err := s.feederGatewayClient.GetCode(v.Address, blockHash, blockNumber)
		if err != nil {
			return
		}
		log.Default.
			With("ContractInfo Address", v.Address, "Block Hash", blockHash, "Block Number", blockNumber).
			Info("Fetched code and ABI")
		// Save the ABI
		services.AbiService.StoreAbi(remove0x(v.Address), code.Abi)
		// Save the contract code
		services.StateService.StoreCode(common.Hex2Bytes(v.Address), code.Bytecode)
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
func parsePages(pages [][]*big.Int) *starknetTypes.StateDiff {
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
	deployedContracts := make([]starknetTypes.DeployedContract, 0)

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
		deployedContracts = append(deployedContracts, starknetTypes.DeployedContract{
			Address:             address,
			ContractHash:        contractHash,
			ConstructorCallData: constructorArguments,
		})
	}
	pagesFlatter = pagesFlatter[deployedContractsInfoLen:]

	// Parse the number of contracts updates
	numContractsUpdate := pagesFlatter[0].Int64()
	pagesFlatter = pagesFlatter[1:]

	storageDiffs := make(map[string][]starknetTypes.KV, 0)

	// Iterate over all the contracts that had been updated and collect the needed information
	for i := int64(0); i < numContractsUpdate; i++ {
		// Parse the Address of the contract
		address := common.Bytes2Hex(pagesFlatter[0].Bytes())
		pagesFlatter = pagesFlatter[1:]

		// Parse the number storage updates
		numStorageUpdates := pagesFlatter[0].Int64()
		pagesFlatter = pagesFlatter[1:]

		kvs := make([]starknetTypes.KV, 0)
		for k := int64(0); k < numStorageUpdates; k++ {
			kvs = append(kvs, starknetTypes.KV{
				Key:   common.Bytes2Hex(pagesFlatter[0].Bytes()),
				Value: common.Bytes2Hex(pagesFlatter[1].Bytes()),
			})
			pagesFlatter = pagesFlatter[2:]
		}
		storageDiffs[address] = kvs
	}

	return &starknetTypes.StateDiff{
		DeployedContracts: deployedContracts,
		StorageDiffs:      storageDiffs,
	}
}

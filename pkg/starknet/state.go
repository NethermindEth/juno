// Package starknet contains all the functions related to Starknet State and Synchronization
// with Layer 2
package starknet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/services"
	base "github.com/NethermindEth/juno/pkg/common"
	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"io/ioutil"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	latestBlockSynced                        = "latestBlockSynced"
	blockOfStarknetDeploymentContractMainnet = 13627000
	blockOfStarknetDeploymentContractGoerli  = 5853000
	MaxChunk                                 = 10000
)

// Synchronizer represents the base struct for Starknet Synchronization
type Synchronizer struct {
	ethereumClient         *ethclient.Client
	feederGatewayClient    *feeder.Client
	db                     *db.Databaser
	MemoryPageHash         base.Dictionary
	GpsVerifier            base.Dictionary
	latestMemoryPageBlock  int64
	latestGpsVerifierBlock int64
	facts                  []string
	stateTrie              trie.Trie
	contractHashes         map[string]*big.Int
	storageTries           map[string]trie.Trie
	lock                   sync.RWMutex
}

// NewSynchronizer creates a new Synchronizer
func NewSynchronizer(db *db.Databaser) *Synchronizer {
	client, err := ethclient.Dial(config.Runtime.Ethereum.Node)
	if err != nil {
		log.Default.With("Error", err).Fatal("Unable to connect to Ethereum Client")
	}
	fClient := feeder.NewClient(config.Runtime.Starknet.FeederGateway, "/feeder_gateway", nil)
	return &Synchronizer{
		ethereumClient:      client,
		feederGatewayClient: fClient,
		db:                  db,
		MemoryPageHash:      base.Dictionary{},
		GpsVerifier:         base.Dictionary{},
		facts:               make([]string, 0),
		stateTrie:           newTrie(),
		contractHashes:      make(map[string]*big.Int),
		storageTries:        make(map[string]trie.Trie),
	}
}

// UpdateState keeps updated the Starknet State in a process
func (s *Synchronizer) UpdateState() error {
	log.Default.Info("Starting to update state")
	if config.Runtime.Starknet.FastSync {
		s.fastSync()
		return nil
	}

	err := s.FetchStarknetState()
	if err != nil {
		return err
	}
	return nil
}

func (s *Synchronizer) initialBlockForStarknetContract() int64 {
	id, err := s.ethereumClient.ChainID(context.Background())
	if err != nil {
		return 0
	}
	if id.Int64() == 1 {
		return blockOfStarknetDeploymentContractMainnet
	}
	return blockOfStarknetDeploymentContractGoerli
}

func (s *Synchronizer) latestBlockQueried() (int64, error) {
	blockNumber, err := (*s.db).Get([]byte(latestBlockSynced))
	if err != nil {
		return 0, err
	}
	if blockNumber == nil {
		return 0, nil
	}
	var ret uint64
	buf := bytes.NewBuffer(blockNumber)
	err = binary.Read(buf, binary.BigEndian, &ret)
	if err != nil {
		return 0, err
	}
	return int64(ret), nil
}

func (s *Synchronizer) updateLatestBlockQueried(block int64) error {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(block))
	err := (*s.db).Put([]byte(latestBlockSynced), b)
	if err != nil {
		log.Default.With("Block", block, "Key", latestBlockSynced).
			Info("Couldn't store the latest synced block")
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

	initialBlock := s.initialBlockForStarknetContract()
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

func (s *Synchronizer) FetchStarknetState() error {
	log.Default.Info("Starting to update state")

	contractAddresses, err := s.feederGatewayClient.GetContractAddresses()
	if err != nil {
		log.Default.With("Error", err).Panic("Couldn't get ContractInfo Address from Feeder Gateway")
		return err
	}
	event := make(chan eventInfo)
	contracts := make(map[common.Address]ContractInfo)

	// Add Starknet contract
	err = loadContractFromDisk(contractAddresses.Starknet,
		config.Runtime.Starknet.ContractAbiPathConfig.StarknetAbiPath,
		"LogStateTransitionFact", contracts)
	if err != nil {
		log.Default.With("Address", contractAddresses.Starknet,
			"Abi Path", config.Runtime.Starknet.ContractAbiPathConfig.StarknetAbiPath).
			Panic("Couldn't load contract from disk ")
		return err
	}
	// Add Starknet contract
	err = loadContractFromDisk(contractAddresses.Starknet,
		config.Runtime.Starknet.ContractAbiPathConfig.StarknetAbiPath,
		"LogStateTransitionFact", contracts)
	if err != nil {
		log.Default.With("Address", contractAddresses.Starknet,
			"Abi Path", config.Runtime.Starknet.ContractAbiPathConfig.StarknetAbiPath).
			Panic("Couldn't load contract from disk ")
		return err
	}

	// Add Gps Statement Verifier contract
	gpsAddress := s.getGpsVerifierAddress()
	err = loadContractFromDisk(gpsAddress,
		config.Runtime.Starknet.ContractAbiPathConfig.GpsVerifierAbiPath,
		"LogMemoryPagesHashes", contracts)
	if err != nil {
		log.Default.With("Address", gpsAddress,
			"Abi Path", config.Runtime.Starknet.ContractAbiPathConfig.GpsVerifierAbiPath).
			Panic("Couldn't load contract from disk ")
		return err
	}
	// Add Memory Page Fact Registry contract
	err = loadContractFromDisk(config.Runtime.Starknet.MemoryPageFactRegistryContract,
		config.Runtime.Starknet.ContractAbiPathConfig.MemoryPageAbiPath,
		"LogMemoryPageFactContinuous", contracts)
	if err != nil {
		log.Default.With("Address", gpsAddress,
			"Abi Path", config.Runtime.Starknet.ContractAbiPathConfig.GpsVerifierAbiPath).
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
				if s.GpsVerifier.Exist(s.facts[0]) {
					s.lock.Lock()
					// If already exist the information related to the fact,
					// fetch the memory pages and updated the State
					go s.processMemoryPages(s.facts[0])
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

				s.lock.Lock()
				s.facts = append(s.facts, common.BytesToHash(b).Hex())
				s.lock.Unlock()

			}

		}
	}
}

// getGpsVerifierAddress returns the address of the GpsVerifierStatement in the current chain
func (s *Synchronizer) getGpsVerifierAddress() string {
	id, err := s.ethereumClient.ChainID(context.Background())
	if err != nil {
		return "0xa739B175325cCA7b71fcB51C3032935Ef7Ac338F"
	}
	if id.Int64() == 1 {
		return "0xa739B175325cCA7b71fcB51C3032935Ef7Ac338F"
	}
	// TODO: Return Goerli Network
	return "0xa739B175325cCA7b71fcB51C3032935Ef7Ac338F"
}

// loadContractFromDisk loads a contract ABI and set the events' thar later we are going yo use
func loadContractFromDisk(contractAddress, abiPath, logName string, contracts map[common.Address]ContractInfo) error {
	// Add Starknet contract
	contractAddressHash := common.HexToAddress(contractAddress)
	contractFromAbi, err := loadContract(abiPath)
	if err != nil {
		return err
	}
	contracts[contractAddressHash] = ContractInfo{
		contract:  contractFromAbi,
		eventName: logName,
	}
	return nil
}

func loadContract(abiPath string) (abi.ABI, error) {
	log.Default.With("ContractInfo", abiPath).Info("Loading contract")
	b, err := ioutil.ReadFile(abiPath)
	if err != nil {
		return abi.ABI{}, err
	}
	contractAbi, err := abi.JSON(strings.NewReader(string(b)))
	if err != nil {
		return abi.ABI{}, err
	}
	return contractAbi, nil
}

// Close closes the client for the Layer 1 Ethereum node
func (s *Synchronizer) Close(ctx context.Context) {
	// notest
	log.Default.Info("Closing Layer 1 Synchronizer")
	select {
	case <-ctx.Done():
		s.ethereumClient.Close()
	default:
	}
}

func (s *Synchronizer) fastSync() {
	latestBlockQueried, err := s.latestBlockQueried()
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
	//go func() {
	log.Default.With("Block Hash", update.BlockHash, "New Root", update.NewRoot, "Old Root", update.OldRoot).
		Info("Updating state")

	upd := convertStateUpdateResponse(update)

	err = s.updateState(upd, update.NewRoot, update.BlockHash, strconv.Itoa(blockIterator))
	if err != nil {
		log.Default.With("Error", err).Panic("Couldn't update state")
	}
	log.Default.With("Block Number", blockIterator).Info("State updated")
	//err = s.updateLatestBlockQueried(int64(blockIterator))
	//if err != nil {
	//	log.Default.With("Error", err).Info("Couldn't save latest block queried")
	//	//return
	//}
	//}()
	return blockIterator + 1, update.BlockHash
}

func newTrie() trie.Trie {
	database := store.New()
	return trie.New(database, 251)
}

func convertStateUpdateResponse(update feeder.StateUpdateResponse) StateDiff {
	var stateDiff StateDiff
	stateDiff.DeployedContracts = make([]DeployedContract, 0)
	stateDiff.StorageDiffs = make(map[string][]KV)
	for _, v := range update.StateDiff.DeployedContracts {
		deployedContract := DeployedContract{
			Address:      v.Address,
			ContractHash: v.ContractHash,
		}
		stateDiff.DeployedContracts = append(stateDiff.DeployedContracts, deployedContract)
	}
	for addressDiff, keyVals := range update.StateDiff.StorageDiffs {
		address := addressDiff
		kvs := make([]KV, 0)
		for _, kv := range keyVals {
			kvs = append(kvs, KV{
				Key:   kv.Key,
				Value: kv.Value,
			})
		}
		stateDiff.StorageDiffs[address] = kvs
	}

	return stateDiff
}

func (s *Synchronizer) updateState(update StateDiff, stateRoot, blockHash, blockNumber string) error {

	for _, deployedContract := range update.DeployedContracts {
		contractHash, ok := new(big.Int).SetString(clean(deployedContract.ContractHash), 16)
		if !ok {
			log.Default.Panic("Couldn't get contract hash")
		}
		s.contractHashes[deployedContract.Address] = contractHash
		storageTrie, ok := s.storageTries[deployedContract.Address]
		if !ok {
			storageTrie = newTrie()
		}
		storageRoot := storageTrie.Commitment()
		address, ok := new(big.Int).SetString(clean(deployedContract.Address), 16)
		if !ok {
			log.Default.With("Address", deployedContract.Address).
				Panic("Couldn't convert Address to Big.Int ")
		}
		contractStateValue := contractState(contractHash, storageRoot)
		s.stateTrie.Put(address, contractStateValue)
		s.storageTries[deployedContract.Address] = storageTrie
	}

	for k, v := range update.StorageDiffs {
		storageTrie, ok := s.storageTries[k]
		if !ok {
			storageTrie = newTrie()
		}
		for _, storageSlots := range v {
			key, ok := new(big.Int).SetString(clean(storageSlots.Key), 16)
			if !ok {
				log.Default.With("Storage Slot Key", storageSlots.Key).
					Panic("Couldn't get the ")
			}
			val, ok := new(big.Int).SetString(clean(storageSlots.Value), 16)
			if !ok {
				log.Default.With("Storage Slot Value", storageSlots.Value).
					Panic("Couldn't get the contract Hash")
			}
			storageTrie.Put(key, val)
		}
		storageRoot := storageTrie.Commitment()
		s.storageTries[k] = storageTrie

		address, ok := new(big.Int).SetString(k[2:], 16)
		if !ok {
			log.Default.With("Address", k).
				Panic("Couldn't convert Address to Big.Int ")
		}
		contractStateValue := contractState(s.contractHashes[k], storageRoot)

		s.stateTrie.Put(address, contractStateValue)
	}

	stateCommitment := clean(s.stateTrie.Commitment().Text(16))

	if stateRoot != "" && stateCommitment != clean(stateRoot) {
		log.Default.With("State Commitment", stateCommitment, "State Root from API", clean(stateRoot)).
			Panic("stateRoot not equal to the one provided")
	}

	log.Default.With("State Root", stateCommitment).
		Info("Got State commitment")

	s.updateAbiAndCode(update, blockHash, blockNumber)
	return nil
}

// contractState define the function that calculates the values stored in the
// leaf of the Merkle Patricia Tree that represent the State in StarkNet
func contractState(contractHash, storageRoot *big.Int) *big.Int {
	// Is defined as:
	// h(h(h(contract_hash, storage_root), 0), 0).
	val, err := pedersen.Digest(contractHash, storageRoot)
	if err != nil {
		log.Default.With("Error", err, "ContractInfo Hash", contractHash.String(),
			"Storage Commitment", storageRoot.String(),
			"Function", "h(contract_hash, storage_root)").
			Panic("Couldn't calculate the digest")
	}
	val, err = pedersen.Digest(val, big.NewInt(0))
	if err != nil {
		log.Default.With("Error", err, "ContractInfo Hash", contractHash.String(),
			"Storage Commitment", storageRoot.String(),
			"Function", "h(h(contract_hash, storage_root), 0)",
			"First Hash", val.String()).
			Panic("Couldn't calculate the digest")
	}
	val, err = pedersen.Digest(val, big.NewInt(0))
	if err != nil {
		log.Default.With("Error", err, "ContractInfo Hash", contractHash.String(),
			"Storage Commitment", storageRoot.String(),
			"Function", "h(h(h(contract_hash, storage_root), 0), 0)",
			"Second Hash", val.String()).
			Panic("Couldn't calculate the digest")
	}
	return val
}

func clean(s string) string {
	answer := ""
	found := false
	for _, char := range s {
		found = found || (char != '0' && char != 'x')
		if found {
			answer = answer + string(char)
		}
	}
	if len(answer) == 0 {
		return "0"
	}
	return answer
}

func (s *Synchronizer) processMemoryPages(fact string) {
	pages := make([][]*big.Int, 0)

	// Get memory pages hashes using fact
	var memoryPages [][32]byte
	memoryPages = (s.GpsVerifier.Get(fact)).([][32]byte)
	memoryContract, err := loadContract(config.Runtime.Starknet.ContractAbiPathConfig.MemoryPageAbiPath)
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
	s.parsePages(pages)
}

type stateToSave struct {
	address       felt.Felt
	contractState struct {
		code    []string
		storage []KV
	}
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
		abiService := services.GetABIService()
		abiService.StoreABI(clean(v.Address), *code.Abi)
		// TODO: Store code and ABI, where to store it? How to store it?

		var address felt.Felt

		// TODO: Save state to trie
		_ = stateToSave{
			address: address,
			contractState: struct {
				code    []string
				storage []KV
			}{
				code.Bytecode, // TODO set how the code is retrieved
				update.StorageDiffs[v.Address],
			},
		}
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
func (s *Synchronizer) parsePages(pages [][]*big.Int) {
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
		address := deployedContractsData[0].String()
		deployedContractsData = deployedContractsData[1:]

		// Parse the ContractInfo Hash
		contractHash := deployedContractsData[0].String()
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
		address := pagesFlatter[0].String()
		pagesFlatter = pagesFlatter[1:]

		// Parse the number storage updates
		numStorageUpdates := pagesFlatter[0].Int64()
		pagesFlatter = pagesFlatter[1:]

		kvs := make([]KV, 0)
		for k := int64(0); k < numStorageUpdates; k++ {
			kvs = append(kvs, KV{
				Key:   pagesFlatter[0].String(),
				Value: pagesFlatter[1].String(),
			})
			pagesFlatter = pagesFlatter[2:]
		}
		storageDiffs[address] = kvs
	}

	state := StateDiff{
		DeployedContracts: deployedContracts,
		StorageDiffs:      storageDiffs,
	}

	log.Default.With("State Diff", state).Info("Fetched state diff")

}

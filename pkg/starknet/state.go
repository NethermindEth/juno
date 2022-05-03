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

const latestBlockSynced = "latestBlockSynced"
const blockOfStarknetDeploymentContractMainnet = 13627000
const blockOfStarknetDeploymentContractGoerli = 5853000
const MaxChunk = 10000

// Synchronizer represents the base struct for Ethereum Synchronization
type Synchronizer struct {
	ethereumClient         *ethclient.Client
	feederGatewayClient    *feeder.Client
	db                     *db.Databaser
	MemoryPageHash         base.Dictionary
	GpsVerifier            base.Dictionary
	latestMemoryPageBlock  int64
	latestGpsVerifierBlock int64
	facts                  []string
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
	}
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
	get, err := (*s.db).Get([]byte(latestBlockSynced))
	if err != nil {
		return 0, err
	}
	if get == nil {
		return 0, nil
	}
	var ret uint64
	buf := bytes.NewBuffer(get)
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

type contractsStruct struct {
	contract  abi.ABI
	eventName string
	address   common.Address
}

type eventStruct struct {
	address         common.Address
	event           map[string]interface{}
	transactionHash common.Hash
}

func (s *Synchronizer) loadEvents(contracts map[common.Address]contractsStruct, eventChan chan eventStruct) error {
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
			eventChan <- eventStruct{
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
			eventChan <- eventStruct{
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
		log.Default.With("Error", err).Info("Couldn't get Contract Address from Feeder Gateway")
		return err
	}
	event := make(chan eventStruct)

	contracts := make(map[common.Address]contractsStruct)

	// Add Starknet contract
	starknetAddress := common.HexToAddress(contractAddresses.Starknet)
	starknetContract, err := loadContract(config.Runtime.Starknet.ContractAbiPathConfig.StarknetAbiPath)
	if err != nil {
		return err
	}
	contracts[starknetAddress] = contractsStruct{
		contract:  starknetContract,
		eventName: "LogStateTransitionFact",
	}

	// Add Gps Statement Verifier contract
	gpsStatementVerifierAddress := common.HexToAddress("0xa739B175325cCA7b71fcB51C3032935Ef7Ac338F")
	gpsStatementVerifierContract, err := loadContract(config.Runtime.Starknet.ContractAbiPathConfig.GpsVerifierAbiPath)
	if err != nil {
		return err
	}
	contracts[gpsStatementVerifierAddress] = contractsStruct{
		contract:  gpsStatementVerifierContract,
		eventName: "LogMemoryPagesHashes",
	}
	// Add Memory Page Fact Registry contract
	memoryPageFactRegistryAddress := common.HexToAddress(config.Runtime.Starknet.MemoryPageFactRegistryContract)
	memoryContract, err := loadContract(config.Runtime.Starknet.ContractAbiPathConfig.MemoryPageAbiPath)
	if err != nil {
		return err
	}
	contracts[memoryPageFactRegistryAddress] = contractsStruct{
		contract:  memoryContract,
		eventName: "LogMemoryPageFactContinuous",
	}
	go func() {

		err = s.loadEvents(contracts, event)
		if err != nil {
			log.Default.With("Error", err).Info("Couldn't get events")
			close(event)
		}
	}()

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

// UpdateState keeps updated the Starknet State in a process
func (s *Synchronizer) UpdateState() error {
	log.Default.Info("Starting to update state")
	//if config.Runtime.Starknet.FastSync {
	//	s.fastSync()
	//	return nil
	//}

	err := s.FetchStarknetState()
	if err != nil {
		return err
	}
	return nil
}

func loadContract(abiPath string) (abi.ABI, error) {
	log.Default.With("Contract", abiPath).Info("Loading contract")
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

	upd := s.convertStateUpdateResponse(update)

	err = s.updateState(upd, update.BlockHash, "")
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't update state")
		//return
	}
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

func (s *Synchronizer) convertStateUpdateResponse(update feeder.StateUpdateResponse) StateDiff {
	var stateDiff StateDiff
	stateDiff.DeployedContracts = make([]DeployedContract, 0)
	stateDiff.StorageDiffs = make(map[common.Address][]KV)
	for _, v := range update.StateDiff.DeployedContracts {
		deployedContract := DeployedContract{
			Address:      common.HexToAddress(v.Address),
			ContractHash: common.HexToHash(v.ContractHash),
		}
		stateDiff.DeployedContracts = append(stateDiff.DeployedContracts, deployedContract)
	}
	for addressDiff, keyVals := range update.StateDiff.StorageDiffs {
		address := common.HexToAddress(addressDiff)
		kvs := make([]KV, 0)
		for _, kv := range keyVals {
			kvs = append(kvs, KV{
				Key:   common.HexToHash(kv.Key),
				Value: common.HexToHash(kv.Value),
			})
		}
		stateDiff.StorageDiffs[address] = kvs
	}

	return stateDiff
}

func (s *Synchronizer) updateState(update StateDiff, blockHash, blockNumber string) error {
	stateRoot := newTrie()
	// Storage root
	storageRoots := make(map[common.Address]trie.Trie)

	contractHashes := make(map[common.Address]common.Hash)
	for _, v := range update.DeployedContracts {
		contractHashes[v.Address] = v.ContractHash
	}

	for k, v := range update.StorageDiffs {
		// Initialization
		if _, ok := storageRoots[k]; !ok {
			// Create new Trie
			storageRoots[k] = newTrie()
		}
		storageTrie, _ := storageRoots[k]
		for _, item := range v {
			keyRaw, _ := new(big.Int).SetString(item.Key.String()[2:], 16)
			valRaw, _ := new(big.Int).SetString(item.Value.String()[2:], 16)
			fmt.Printf("Put(%s, %s)", item.Key, item.Value)
			storageTrie.Put(keyRaw, valRaw)
		}
		storageRoot := storageTrie.Commitment()
		log.Default.With("Storage Root", storageRoot.Text(16),
			"Contract Address", k).
			Info("Storage commitment")
		// h(h(h(contract_hash,storage_root), 0), 0)
		contractHash, _ := new(big.Int).SetString(contractHashes[k].String()[2:], 16)
		// Pedersen Hash of (contract_hash, storage_root)
		p1, err := pedersen.Digest(contractHash, storageRoot)
		if err != nil {
			log.Default.With("Error", err).Info("Couldn't use digest")
		}
		// Pedersen Hash of h(contract_hash, storage_root)
		p2, err := pedersen.Digest(p1, big.NewInt(0))
		if err != nil {
			log.Default.With("Error", err).Info("Couldn't use digest")
		}
		// Pedersen Hash of (contract_hash, storage_root)
		leafValue, err := pedersen.Digest(p2, big.NewInt(0))
		if err != nil {
			log.Default.With("Error", err).Info("Couldn't use digest")
		}
		leafKey, _ := new(big.Int).SetString(k.String()[2:], 16)
		stateRoot.Put(leafKey, leafValue)
	}

	log.Default.With("State Root", stateRoot.Commitment().Text(16)).
		Info("Got State commitment")

	s.updateAbiAndCode(update, blockHash, blockNumber)
	return nil
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
		code, err := s.feederGatewayClient.GetCode(v.Address.String(), blockHash, blockNumber)
		if err != nil {
			return
		}
		log.Default.With("Contract Address", v.Address, "Block Hash", blockHash, "Block Number",
			blockNumber, "Code", code).
			Info("Got code and ABI")
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

// KV represents a key-value pair.
type KV struct {
	Key   common.Hash `json:"key"`
	Value common.Hash `json:"value"`
}

// DeployedContract represent the contracts that have been deployed in this block
// and the information stored on-chain
type DeployedContract struct {
	Address             common.Address `json:"address"`
	ContractHash        common.Hash    `json:"contract_hash"`
	ConstructorCallData []*big.Int     `json:"constructor_call_data"`
}

// StateDiff Represent the deployed contracts and the storage diffs for those and
// for the one's already deployed
type StateDiff struct {
	DeployedContracts []DeployedContract      `json:"deployed_contracts"`
	StorageDiffs      map[common.Address][]KV `json:"storage_diffs"`
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
		address := common.BytesToAddress(deployedContractsData[0].Bytes())
		deployedContractsData = deployedContractsData[1:]

		// Parse the Contract Hash
		contractHash := common.BytesToHash(deployedContractsData[0].Bytes())
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

		// Store deployed Contract information
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

	storageDiffs := make(map[common.Address][]KV, 0)

	// Iterate over all the contracts that had been updated and collect the needed information
	for i := int64(0); i < numContractsUpdate; i++ {
		// Parse the Address of the contract
		address := common.BytesToAddress(pagesFlatter[0].Bytes())
		pagesFlatter = pagesFlatter[1:]

		// Parse the number storage updates
		numStorageUpdates := pagesFlatter[0].Int64()
		pagesFlatter = pagesFlatter[1:]

		kvs := make([]KV, 0)
		for k := int64(0); k < numStorageUpdates; k++ {
			key := common.BytesToHash(pagesFlatter[0].Bytes())
			value := common.BytesToHash(pagesFlatter[1].Bytes())
			kvs = append(kvs, KV{
				Key:   key,
				Value: value,
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

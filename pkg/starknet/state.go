// Package starknet contains all the functions related to Starknet State and Synchronization
// with Layer 2
package starknet

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/services"
	common2 "github.com/NethermindEth/juno/pkg/common"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/starknet/abi"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
	"github.com/ethereum/go-ethereum"
	ethAbi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
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
		// notest
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

// UpdateState initiates network syncing. Syncing will occur against the
// feeder gateway or Layer 1 depending on the configuration.
// notest
func (s *Synchronizer) UpdateState() error {
	log.Default.Info("Starting to update state")
	if config.Runtime.Starknet.ApiSync {
		return s.apiSync()
	}
	return s.l1Sync()
}

// loadEvents sends all logs ever emitted by `contracts` and adds them
// to `eventChan`. Once caught up with the main chain, it will listen
// for events originating from `contracts` indefinitely.
// notest
func (s *Synchronizer) loadEvents(
	contracts map[common.Address]starknetTypes.ContractInfo,
	eventChan chan starknetTypes.EventInfo,
) error {
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
			FromBlock: new(big.Int).SetUint64(i),
			ToBlock:   new(big.Int).SetUint64(i + increment),
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

// l1Sync syncs against the starknet data stored on layer 1. It calls
// `loadEvents` to obtain events from three of the Starknet contracts on
// Ethereum:
//
// 1. MemoryPageFactRegistry: stores a mapping between a fact (usually
// a hash of some data) and a memory page hash. A memory page is the
// memory of a Cairo contract.
//
// 2. GpsStatementVerifier: verifies proofs from layer 2. We listen for
// the `LogMemoryPagesHashes` event, which contains a fact and an array
// memory pages' hashes. This log is emitted after the proof for a set
// of Cairo transactions is verified.
//
// 3. Starknet: responsible for Starknet state transitions. It emits a
// `LogStateTransitionFact` event with a fact corresponding to the
// state transition being processed. Once it completes additional
// safety checks, it will officially transition the state and emit a
// `LogStateUpdate` event with the new state root and Starknet block
// number (sequence number).
//
// Once this function sees a `LogStateTransitionFact` event, it works
// backward through the steps above to reconstruct the original Starknet
// transactions.
// notest
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
			if !s.facts.Exist(strconv.FormatUint(factSynced, 10)) {
				continue
			}
			f, _ := s.facts.Get(strconv.FormatUint(factSynced, 10), starknetTypes.Fact{})
			fact := f.(starknetTypes.Fact)

			if s.gpsVerifier.Exist(fact.Value) {
				// Get memory pages hashes using fact
				pagesHashes, err := s.gpsVerifier.Get(fact.Value, starknetTypes.PagesHash{})
				if err != nil {
					log.Default.With("Error").Panic("Fact has not been verified")
				}
				// If already exist the information related to the fact,
				// fetch the memory pages and updated the State
				pages := s.processPagesHashes(
					pagesHashes.(starknetTypes.PagesHash).Bytes,
					contracts[common.HexToAddress(memoryPagesContractAddress)].Contract,
				)

				stateDiff := parsePages(pages)

				// Update state
				s.updateAndCommitState(stateDiff, fact.StateRoot, fact.SequenceNumber)

				err = updateNumericValueFromDB(s.database, starknetTypes.LatestFactSynced, factSynced)
				if err != nil {
					log.Default.With("Error", err).Info("Couldn't update latest block synced")
				}

				s.facts.Remove(strconv.FormatUint(factSynced, 10))
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
			contractAbi, _ := loadAbiOfContract(abi.StarknetAbi)
			starknetAddress := common.HexToAddress(contractAddresses.Starknet)

			factSaved, err := getNumericValueFromDB(s.database, starknetTypes.LatestFactSaved)
			if err != nil {
				log.Default.With("Error", err).
					Info("Unable to get the Value of the latest fact synced")
				return err
			}

			blockNumber := new(big.Int).SetUint64(l.Block)
			query := ethereum.FilterQuery{
				FromBlock: blockNumber,
				ToBlock:   blockNumber,
				Addresses: []common.Address{starknetAddress},
				Topics:    [][]common.Hash{{crypto.Keccak256Hash([]byte(contractAbi.Events["LogStateUpdate"].Sig))}},
			}

			starknetLogs, err := s.ethereumClient.FilterLogs(context.Background(), query)
			if err != nil {
				log.Default.With("Error", err, "Initial block", l.Block, "End block", l.Block+1).
					Info("Couldn't get logs")
			}
			fullFact, _ := getFactInfo(starknetLogs, contractAbi, common.BytesToHash(b).Hex(), factSaved)

			// Safe Fact for block x
			s.facts.Add(strconv.FormatUint(factSaved, 10), fullFact)

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

// updateAndCommitState applies `stateDiff` to the local state and
// commits the changes to the database.
// notest
func (s *Synchronizer) updateAndCommitState(
	stateDiff *starknetTypes.StateDiff,
	newRoot string,
	sequenceNumber uint64,
) {
	txn := s.transactioner.Begin()
	hashService := services.GetContractHashService()
	if hashService == nil {
		log.Default.Panic("Contract hash service is unavailable")
	}
	_, err := updateState(txn, services.GetContractHashService(), stateDiff, newRoot, sequenceNumber)
	if err != nil {
		log.Default.With("Error", err).Panic("Couldn't update state")
	} else {
		err := txn.Commit()
		if err != nil {
			log.Default.Panic("Couldn't commit to the database")
		}
	}
	log.Default.With("Block Number", sequenceNumber).Info("State updated")

	err = updateNumericValueFromDB(s.database, starknetTypes.LatestBlockSynced, sequenceNumber)
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't save latest block queried")
	}
}

// getFactInfo gets the state root and sequence number associated with
// a given StateTransitionFact.
// notest
func getFactInfo(
	starknetLogs []types.Log,
	contract ethAbi.ABI,
	fact string,
	latestBlockSynced uint64,
) (*starknetTypes.Fact, error) {
	for _, vLog := range starknetLogs {
		log.Default.With("Log Fetched", "LogStateUpdate", "BlockHash", vLog.BlockHash.Hex(),
			"BlockNumber", vLog.BlockNumber, "TxHash", vLog.TxHash.Hex())
		event := map[string]interface{}{}
		err := contract.UnpackIntoMap(event, "LogStateUpdate", vLog.Data)
		if err != nil {
			log.Default.With("Error", err).Info("Couldn't get state root or sequence number from LogStateUpdate event")
			return nil, err
		}
		factVal := &starknetTypes.Fact{
			StateRoot:      common.BigToHash(event["globalRoot"].(*big.Int)).String(),
			SequenceNumber: event["blockNumber"].(*big.Int).Uint64(),
			Value:          fact,
		}
		if factVal.SequenceNumber == latestBlockSynced {
			return factVal, nil
		}
	}
	log.Default.Panic("Couldn't find a block number that match in the logs for given fact")
	return nil, nil
}

// Close closes the client for the Layer 1 Ethereum node
func (s *Synchronizer) Close(ctx context.Context) {
	// notest
	log.Default.Info("Closing Layer 1 Synchronizer")
	s.ethereumClient.Close()
	//(*s.database).Close()
}

// apiSync syncs against the feeder gateway.
// notest
func (s *Synchronizer) apiSync() error {
	blockIterator, err := getNumericValueFromDB(s.database, starknetTypes.LatestBlockSynced)
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't get latest Block queried")
		return err
	}
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

// updateStateForOneBlock will fetch state transition from the feeder
// gateway and apply it to the local state.
// notest
func (s *Synchronizer) updateStateForOneBlock(blockIterator uint64, lastBlockHash string) (uint64, string) {
	log.Default.With("Number", blockIterator).Info("Updating StarkNet State")
	update, err := s.feederGatewayClient.GetStateUpdate("", strconv.FormatUint(blockIterator, 10))
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

	upd := stateUpdateResponseToStateDiff(*update)

	s.updateAndCommitState(&upd, update.NewRoot, blockIterator)

	s.updateServices(upd, lastBlockHash, strconv.FormatUint(blockIterator, 10))

	return blockIterator + 1, update.BlockHash
}

// processPagesHashes takes an arrays of arrays of pages' hashes and
// converts them into memory pages by querying an ethereum client.
// notest
func (s *Synchronizer) processPagesHashes(pagesHashes [][32]byte, memoryContract ethAbi.ABI) [][]*big.Int {
	pages := make([][]*big.Int, 0)
	for _, v := range pagesHashes {
		// Get transactionsHash based on the memory page
		hash := common.Bytes2Hex(v[:])
		transactionHash, err := s.memoryPageHash.Get(hash, starknetTypes.TransactionHash{})
		txHash := transactionHash.(starknetTypes.TransactionHash).Hash
		if err != nil {
			return nil
		}
		log.Default.With("Hash", txHash.Hex()).Info("Getting transaction...")
		txn, _, err := s.ethereumClient.TransactionByHash(context.Background(), txHash)
		if err != nil {
			log.Default.With("Error", err, "Transaction Hash", v).
				Error("Couldn't retrieve transactions")
			return nil
		}

		// Parse Ethereum transaction calldata for Starknet transaction information
		data := txn.Data()[4:] // Remove the method signature hash
		inputs := make(map[string]interface{})
		err = memoryContract.Methods["registerContinuousMemoryPage"].Inputs.UnpackIntoMap(inputs, data)
		if err != nil {
			log.Default.With("Error", err).Info("Couldn't unpack into map")
			return nil
		}
		// Append calldata to pages
		pages = append(pages, inputs["values"].([]*big.Int))
	}
	return pages
}

// notest
func (s *Synchronizer) updateServices(update starknetTypes.StateDiff, blockHash, blockNumber string) {
	s.updateAbiAndCode(update, blockHash, blockNumber)
	s.updateBlocksAndTransactions(blockHash, blockNumber)
}

// notest
func (s *Synchronizer) updateAbiAndCode(update starknetTypes.StateDiff, blockHash string, sequenceNumber string) {
	for _, v := range update.DeployedContracts {
		code, err := s.feederGatewayClient.GetCode(v.Address, blockHash, sequenceNumber)
		if err != nil {
			return
		}
		log.Default.
			With("ContractInfo Address", v.Address, "Block Hash", blockHash, "Block Number", sequenceNumber).
			Info("Fetched code and ABI")
		// TODO: Convert ABI and Code in Database
		// Save the ABI
		services.AbiService.StoreAbi(remove0x(v.Address), toDbAbi(code.Abi))
		// Save the contract code
		services.StateService.StoreCode(common.Hex2Bytes(v.Address), byteCodeToStateCode(code.Bytecode))
	}
}

// notest
func (s *Synchronizer) updateBlocksAndTransactions(blockHash, blockNumber string) {
	block, err := s.feederGatewayClient.GetBlock(blockHash, blockNumber)
	if err != nil {
		return
	}
	log.Default.With("Block Hash", blockHash).
		Info("Got block")
	services.BlockService.StoreBlock(common.Hex2Bytes(blockHash), feederBlockToDBBlock(block))

	for _, bTxn := range block.Transactions {
		transactionInfo, err := s.feederGatewayClient.GetTransaction(bTxn.TransactionHash, "")
		if err != nil {
			return
		}
		log.Default.With("Transaction Hash", transactionInfo.Transaction.TransactionHash).
			Info("Got transactions of block")
		services.TransactionService.StoreTransaction(common2.HexToFelt(bTxn.TransactionHash).Bytes(),
			feederTransactionToDBTransaction(transactionInfo))
	}
}

// parsePages converts an array of memory pages into a state diff that
// can be used to update the local state.
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

package starknet

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	contractAbis "github.com/NethermindEth/juno/pkg/starknet_new/contracts"
	"github.com/NethermindEth/juno/pkg/types"
	eth "github.com/ethereum/go-ethereum"
	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type ethereumClient interface {
	TransactionByHash(ctx context.Context, hash ethcommon.Hash) (tx *ethtypes.Transaction, isPending bool, err error)
	FilterLogs(ctx context.Context, q eth.FilterQuery) ([]ethtypes.Log, error)
	SubscribeFilterLogs(ctx context.Context, q eth.FilterQuery, ch chan<- ethtypes.Log) (eth.Subscription, error)
	BlockNumber(ctx context.Context) (uint64, error)
}

type PagesHashes [][32]byte

type L1Config struct {
	ChainId                   uint64
	StarknetDeploymentBlock   uint64
	Contracts                 map[ethcommon.Address]contractInfo
	StarknetAddress           ethcommon.Address
	MemoryPageRegistryAddress ethcommon.Address
	GpsVerifierAddress        ethcommon.Address
}

type l1StateUpdate struct {
	Fact    ethcommon.Hash
	NewRoot string
}

type contractInfo struct {
	ABI       ethabi.ABI
	EventName string
}

type Contracts map[string]contractInfo

func InitL1Config(chainId uint64, feederClient *feeder.Client) (*L1Config, error) {
	// Goerli
	memoryPageRegistryAddr := "0x743789ff2ff82bfb907009c9911a7da636d34fa7"
	starknetDeploymentBlock := 5853000
	if chainId == 1 {
		// Mainnet
		memoryPageRegistryAddr = "0x96375087b2f6efc59e5e0dd5111b4d090ebfdd8b"
		starknetDeploymentBlock = 13627000
	}
	memoryPageRegistryAddress := ethcommon.HexToAddress(memoryPageRegistryAddr)

	starknetAbi, err := ethabi.JSON(strings.NewReader(contractAbis.StarknetAbiJson))
	if err != nil {
		return nil, err
	}
	memoryPagesAbi, err := ethabi.JSON(strings.NewReader(contractAbis.MemoryPagesAbiJson))
	if err != nil {
		return nil, err
	}
	gpsVerifierAbi, err := ethabi.JSON(strings.NewReader(contractAbis.GpsVerifierAbiJson))
	if err != nil {
		return nil, err
	}

	contractAddresses, err := feederClient.GetContractAddresses()
	if err != nil {
		return nil, err
	}
	starknetAddress := ethcommon.HexToAddress(contractAddresses.Starknet)
	gpsVerifierAddress := ethcommon.HexToAddress(contractAddresses.GpsStatementVerifier)

	return &L1Config{
		ChainId:                 chainId,
		StarknetDeploymentBlock: uint64(starknetDeploymentBlock),

		Contracts: map[ethcommon.Address]contractInfo{
			memoryPageRegistryAddress: {
				ABI:       memoryPagesAbi,
				EventName: "LogMemoryPageFactContinuous",
			},
			starknetAddress: {
				ABI:       starknetAbi,
				EventName: "LogStateTransitionFact",
			},
			gpsVerifierAddress: {
				ABI:       gpsVerifierAbi,
				EventName: "LogMemoryPagesHashes",
			},
		},

		StarknetAddress:           starknetAddress,
		GpsVerifierAddress:        gpsVerifierAddress,
		MemoryPageRegistryAddress: memoryPageRegistryAddress,
	}, nil
}

func L1LoadStateDiffs(nextBlock uint64, ethclient ethereumClient, config *L1Config, stateDiffsChan chan *types.StateUpdate, errChan chan error) {
	defer close(stateDiffsChan)

	memoryPageToTxHash := NewDictionary()
	factToPageHash := NewDictionary()
	blockNumToFact := NewDictionary()

	// Latest block
	latestEthBlockNumber, err := ethclient.BlockNumber(context.Background())
	if err != nil {
		log.Default.With("error", err).Error("failed get the latest ethereum block number from ethereum client")
		return
	}

	// Poll
	query := eth.FilterQuery{
		Addresses: []ethcommon.Address{config.GpsVerifierAddress, config.MemoryPageRegistryAddress, config.StarknetAddress},
		Topics:    [][]ethcommon.Hash{{getTopic(config.Contracts[config.StarknetAddress], ""), getTopic(config.Contracts[config.GpsVerifierAddress], ""), getTopic(config.Contracts[config.MemoryPageRegistryAddress], "")}},
	}
	bigFromBlock := new(big.Int)
	bigToBlock := new(big.Int)

	nextBlockNumber := nextBlock
	nextFact := nextBlock
	// TODO send ethblock to service
	for startBlock, endBlock := config.StarknetDeploymentBlock, config.StarknetDeploymentBlock+10_000; endBlock < latestEthBlockNumber; startBlock, endBlock = endBlock, endBlock+10_000 {
		query.FromBlock = bigFromBlock.SetUint64(startBlock)
		query.ToBlock = bigToBlock.SetUint64(endBlock)
		starknetLogs, err := ethclient.FilterLogs(context.Background(), query)
		if err != nil {
			log.Default.With("error", err, "initial block", startBlock, "end block", endBlock).Error("couldn't get logs")
			return
		}
		log.Default.With("count", len(starknetLogs)).Info("fetched logs")

		nextFact, err = processLogs(starknetLogs, config, blockNumToFact, factToPageHash, memoryPageToTxHash, ethclient, nextFact)
		if err != nil {
			panic(err)
		}
		nextBlockNumber, _ = createStateUpdate(nextBlockNumber, blockNumToFact, factToPageHash, memoryPageToTxHash, ethclient, config.Contracts[config.MemoryPageRegistryAddress].ABI, stateDiffsChan)
	}
	// TODO do we need to poll again? from endblock to the latest block?

	// Subscribe
	query.FromBlock = nil
	query.ToBlock = nil
	hLog := make(chan ethtypes.Log, 2000)
	sub, err := ethclient.SubscribeFilterLogs(context.Background(), query, hLog)
	defer sub.Unsubscribe()
	if err != nil {
		log.Default.With("error", err).Error("failed to subscribe to ethereum client for logs")
		return
	}
	log.Default.Info("subscribed to L1 client")

	for {
		select {
		case err := <-errChan:
			log.Default.With("error", err).Info("unexpected error")
			return
		case err := <-sub.Err():
			log.Default.With("error", err).Info("error getting the latest logs")
			return
		case vLog := <-hLog:
			processLogs([]ethtypes.Log{vLog}, config, blockNumToFact, factToPageHash, memoryPageToTxHash, ethclient, nextBlockNumber)
			if err != nil {
				panic(err)
			}
			nextBlockNumber, _ = createStateUpdate(nextBlockNumber, blockNumToFact, factToPageHash, memoryPageToTxHash, ethclient, config.Contracts[config.MemoryPageRegistryAddress].ABI, stateDiffsChan)
		}
	}
}

func getFactInfo(starknetLogs []ethtypes.Log, contract ethabi.ABI, latestFactSaved uint64, txHash ethcommon.Hash) (string, error) {
	for _, vLog := range starknetLogs {
		// Corresponding LogStateUpdate for the LogStateTransitionFact (they must occur in the same transaction)
		if vLog.TxHash.Hex() == txHash.Hex() {
			event := map[string]interface{}{}
			if err := contract.UnpackIntoMap(event, "LogStateUpdate", vLog.Data); err != nil {
				log.Default.With("error", err).Info("failed to unpack LogStateUpdate event")
				continue
			}
			sequenceNumber := event["blockNumber"].(*big.Int).Uint64()
			// If we are caught up to the blocks in the database, this will be true.
			// If we are catching up, this will be false.
			if sequenceNumber == latestFactSaved {
				log.Default.With("sequence number", sequenceNumber).Info("found LogStateUpdate")
				stateRoot := event["globalRoot"].(*big.Int).Text(16)
				return stateRoot, nil
			} else {
				return "", errors.New("catching up to saved state")
			}
		}
	}
	return "", errors.New("couldn't find a block number in logs for given fact")
}

// stateDiffFromPages converts an array of memory pages into a state diff that
// can be used to update the local state.
func stateDiffFromPages(pages [][]*big.Int) *types.StateDiff {
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
	deployedContracts := make([]types.DeployedContract, 0)

	// Get the info of the deployed contracts
	deployedContractsData := pagesFlatter[:deployedContractsInfoLen]

	// Iterate while contains contract data to be processed
	for len(deployedContractsData) > 0 {
		// Parse the Address of the contract
		address := ethcommon.Bytes2Hex(deployedContractsData[0].Bytes())
		deployedContractsData = deployedContractsData[1:]

		// Parse the ContractInfo Hash
		contractHash := ethcommon.Bytes2Hex(deployedContractsData[0].Bytes())
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
		deployedContracts = append(deployedContracts, types.DeployedContract{
			Address:             address,
			Hash:                contractHash,
			ConstructorCallData: constructorArguments,
		})
	}
	pagesFlatter = pagesFlatter[deployedContractsInfoLen:]

	// Parse the number of contracts updates
	numContractsUpdate := pagesFlatter[0].Int64()
	pagesFlatter = pagesFlatter[1:]

	storageDiffs := make(map[string][]types.MemoryCell, 0)

	// Iterate over all the contracts that had been updated and collect the needed information
	for i := int64(0); i < numContractsUpdate; i++ {
		// Parse the Address of the contract
		address := ethcommon.Bytes2Hex(pagesFlatter[0].Bytes())
		pagesFlatter = pagesFlatter[1:]

		// Parse the number storage updates
		numStorageUpdates := pagesFlatter[0].Int64()
		pagesFlatter = pagesFlatter[1:]

		kvs := make([]types.MemoryCell, 0)
		for k := int64(0); k < numStorageUpdates; k++ {
			kvs = append(kvs, types.MemoryCell{
				Address: ethcommon.Bytes2Hex(pagesFlatter[0].Bytes()),
				Value:   ethcommon.Bytes2Hex(pagesFlatter[1].Bytes()),
			})
			pagesFlatter = pagesFlatter[2:]
		}
		storageDiffs[address] = kvs
	}

	return &types.StateDiff{
		DeployedContracts: deployedContracts, StorageDiffs: storageDiffs,
	}
}

func processLogs(vLogs []ethtypes.Log, config *L1Config, facts *dictionary, factToPageHash *dictionary, memoryPageToTxHash *dictionary, ethclient ethereumClient, nextFact uint64) (uint64, error) {
	for _, vLog := range vLogs {
		if vLog.Removed {
			continue
		}
		contract := config.Contracts[vLog.Address]
		log.Default.With("Log Fetched", contract.EventName, "BlockHash", vLog.BlockHash.Hex(), "BlockNumber", vLog.BlockNumber, "TxHash", vLog.TxHash.Hex()).Info("Event Fetched")
		event := map[string]interface{}{}
		err := contract.ABI.UnpackIntoMap(event, contract.EventName, vLog.Data)
		if err != nil {
			log.Default.With("Error", err).Info("Couldn't get event from log")
			return nextFact, err
		}

		switch contract.EventName {
		case config.Contracts[config.MemoryPageRegistryAddress].EventName:
			memoryPageToTxHash.Add(event["memoryHash"].(*big.Int).Text(16), vLog.TxHash)
		case config.Contracts[config.GpsVerifierAddress].EventName:
			fact := event["factHash"].([32]byte)
			factToPageHash.Add("0x"+ethcommon.Bytes2Hex(fact[:]), event["pagesHashes"].(PagesHashes))
		case config.Contracts[config.StarknetAddress].EventName:
			tmp := event["stateTransitionFact"].([32]byte)
			fact := ethcommon.BytesToHash(tmp[:])
			ethBlockNumber := new(big.Int).SetUint64(vLog.BlockNumber)
			query := eth.FilterQuery{
				FromBlock: ethBlockNumber,
				ToBlock:   ethBlockNumber,
				Addresses: []ethcommon.Address{vLog.Address},
				Topics:    [][]ethcommon.Hash{{getTopic(contract, "LogStateUpdate")}},
			}
			starknetLogs, err := ethclient.FilterLogs(context.Background(), query)
			if err != nil {
				log.Default.With("error", err).Error("ethereum client failed to return LogStateUpdate logs")
				return nextFact, err
			}

			newRoot, err := getFactInfo(starknetLogs, contract.ABI, nextFact, vLog.TxHash)
			if err != nil {
				log.Default.With("error", err).Error("could not find LogStateUpdate")
				return nextFact, err
			}
			facts.Add(nextFact, l1StateUpdate{Fact: fact, NewRoot: newRoot})
			nextFact++
		}
	}
	return nextFact, nil
}

func getTopic(contract contractInfo, eventName string) ethcommon.Hash {
	if eventName == "" {
		eventName = contract.EventName
	}
	return ethcrypto.Keccak256Hash([]byte(contract.ABI.Events[eventName].Sig))
}

func createStateUpdate(startBlockNum uint64, facts *dictionary, factToPageHash *dictionary, memoryPageToTxHash *dictionary, ethclient ethereumClient, memoryContract ethabi.ABI, stateDiffsChan chan *types.StateUpdate) (uint64, error) {
	// Send state updates until we don't have enough information to construct one
	for nextBlockNumber := startBlockNum; ; nextBlockNumber++ {
		l1StateUpdateInterface, ok := facts.Get(nextBlockNumber)
		if !ok {
			return nextBlockNumber, nil
		}
		stateUpdate := l1StateUpdateInterface.(l1StateUpdate)

		pagesHashesInterface, ok := factToPageHash.Get(stateUpdate.Fact.Hex())
		if !ok {
			return nextBlockNumber, nil
		}
		pagesHashes := pagesHashesInterface.(PagesHashes)

		for _, pageHash := range pagesHashes {
			if !memoryPageToTxHash.Exist(pageHash) {
				return nextBlockNumber, nil
			}
		}

		// Now we have everything we need to construct a state update object
		txs, err := txsFromPagesHashes(memoryPageToTxHash, ethclient)
		if err != nil {
			log.Default.With("error", err).Error("failed to receive memory page transactions from ethereum client")
			return nextBlockNumber, nil
		}

		pages, err := pagesFromTxs(txs, memoryContract)
		if err != nil {
			log.Default.With("error", err).Error("failed to parse memory page transaction calldata")
			return nextBlockNumber, err
		}

		stateDiffsChan <- &types.StateUpdate{
			StateDiff:      stateDiffFromPages(pages),
			NewRoot:        stateUpdate.NewRoot,
			NewBlockNumber: nextBlockNumber,
		}
		// TODO delete processed events
	}
}

func txsFromPagesHashes(pagesHashesToTxHashes *dictionary, ethclient ethereumClient) ([]*ethtypes.Transaction, error) {
	numTxs := len(pagesHashesToTxHashes.m)
	txs := make([]*ethtypes.Transaction, numTxs)

	errChan := make(chan error)
	defer close(errChan)

	var g errgroup.Group

	i := 0
	for _, transactionHash := range pagesHashesToTxHashes.m {
		g.Go(func() error {
			txHash := transactionHash.(ethcommon.Hash)
			tx, _, err := ethclient.TransactionByHash(context.Background(), txHash)
			// TODO handle isPending?
			if err != nil {
				return err
			}
			txs[i] = tx
			return nil
		})
		i++
	}

	return txs, g.Wait()
}

func pagesFromTxs(txs []*ethtypes.Transaction, memoryContract ethabi.ABI) ([][]*big.Int, error) {
	pages := make([][]*big.Int, len(txs)) // TODO uint256 library here?
	for i, tx := range txs {
		// Parse Ethereum transaction calldata for Starknet transaction information
		data := tx.Data()[4:] // Remove the method signature hash
		inputs := make(map[string]interface{})
		err := memoryContract.Methods["registerContinuousMemoryPage"].Inputs.UnpackIntoMap(inputs, data)
		if err != nil {
			log.Default.With("error", err).Info("couldn't unpack into map")
			return nil, err
		}
		// Append calldata to pages
		pages[i] = inputs["values"].([]*big.Int)
	}
	return pages, nil
}

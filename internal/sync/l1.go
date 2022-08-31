package sync

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/NethermindEth/juno/internal/db"
	syncdb "github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/sync/contracts"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type ContractConfig struct {
	address         ethcommon.Address
	deploymentBlock uint64
}

// TODO this should probably be in the config package
type L1ChainConfig struct {
	starknet               ContractConfig
	gpsStatementVerifier   ContractConfig
	memoryPageFactRegistry ContractConfig
}

type l1Reader interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*ethtypes.Header, error)
	TransactionByHash(ctx context.Context, txHash ethcommon.Hash) (tx *ethtypes.Transaction, isPending bool, err error)
}

type L1Backend interface {
	bind.ContractBackend
	l1Reader
}

type L1SyncService struct {
	// L1 StarkNet Contract bindings
	starknet               *contracts.StarknetContract
	gpsStatementVerifier   *contracts.GpsStatementVerifierContract
	memoryPageFactRegistry *contracts.MemoryPageFactRegistryContract

	manager  *syncdb.Manager
	l1Reader l1Reader

	state     state.State
	nextBlock uint64

	// An in-memory priority queue of the state updates not yet
	// committed to the database.
	queue *StateUpdateQueue
}

func NewL1SyncService(chain L1ChainConfig, backend L1Backend, syncManager *syncdb.Manager, stateManager state.StateManager) (*L1SyncService, error) {
	starknet, err := contracts.NewStarknetContract(chain.starknet.address, chain.starknet.deploymentBlock, backend)
	if err != nil {
		return nil, err
	}
	gpsStatementVerifier, err := contracts.NewGpsStatementVerifierContract(chain.gpsStatementVerifier.address, chain.gpsStatementVerifier.deploymentBlock, backend)
	if err != nil {
		return nil, err
	}
	memoryPageFactRegistry, err := contracts.NewMemoryPageFactRegistryContract(chain.memoryPageFactRegistry.address, chain.memoryPageFactRegistry.deploymentBlock, backend)
	if err != nil {
		return nil, err
	}

	return &L1SyncService{
		starknet:               starknet,
		gpsStatementVerifier:   gpsStatementVerifier,
		memoryPageFactRegistry: memoryPageFactRegistry,
		l1Reader:               backend,
		manager:                syncManager,
		state:                  state.New(stateManager, new(felt.Felt)),
		queue:                  NewStateUpdateQueue(),
		nextBlock:              0, // TODO we should get this from the sync manager
	}, nil
}

// TODO get more accurate deployment blocks

func NewMainnetL1SyncService(backend L1Backend, syncManager *syncdb.Manager, stateManager state.StateManager) (*L1SyncService, error) {
	chain := L1ChainConfig{
		starknet: ContractConfig{
			address:         ethcommon.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
			deploymentBlock: 13617000,
		},
		gpsStatementVerifier: ContractConfig{
			address:         ethcommon.HexToAddress("0xa739b175325cca7b71fcb51c3032935ef7ac338f"),
			deploymentBlock: 0,
		},
		memoryPageFactRegistry: ContractConfig{
			address:         ethcommon.HexToAddress("0x96375087b2f6efc59e5e0dd5111b4d090ebfdd8b"),
			deploymentBlock: 0,
		},
	}
	return NewL1SyncService(chain, backend, syncManager, stateManager)
}

func NewGoerliL1SyncService(backend L1Backend, syncManager *syncdb.Manager, stateManager state.StateManager) (*L1SyncService, error) {
	chain := L1ChainConfig{
		starknet: ContractConfig{
			address:         ethcommon.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e"),
			deploymentBlock: 5840000,
		},
		gpsStatementVerifier: ContractConfig{
			address:         ethcommon.HexToAddress("0x5ef3c980bf970fce5bbc217835743ea9f0388f4f"),
			deploymentBlock: 0,
		},
		memoryPageFactRegistry: ContractConfig{
			address:         ethcommon.HexToAddress("0x743789ff2ff82bfb907009c9911a7da636d34fa7"),
			deploymentBlock: 0,
		},
	}
	return NewL1SyncService(chain, backend, syncManager, stateManager)
}

func (s *L1SyncService) Run(errChan chan error) {
	logStateUpdates := make(chan *contracts.StarknetLogStateUpdate)
	go s.getLogStateUpdates(logStateUpdates, errChan)

	for logStateUpdate := range logStateUpdates {
		fmt.Printf("found log state update: %d\n", logStateUpdate.BlockNumber)
		go func(logStateUpdate *contracts.StarknetLogStateUpdate) {
			if err := s.updateState(logStateUpdate); err != nil {
				errChan <- err
			}
		}(logStateUpdate)
	}
}

func (s *L1SyncService) Close() error {
	return nil // TODO
}

func (s *L1SyncService) updateState(logStateUpdate *contracts.StarknetLogStateUpdate) error {
	logStateTransitionFact, err := s.findLogStateTransitionFact(logStateUpdate.Raw.BlockNumber, logStateUpdate.Raw.TxHash)
	if err != nil {
		return err
	}

	pagesHashes, blockNum, err := s.findLogMemoryPagesHashes(logStateTransitionFact.Raw.BlockNumber, logStateTransitionFact.StateTransitionFact, 500)
	if err != nil {
		return err
	}

	logMemoryPageFactContinuouses, err := s.findLogMemoryPageFactContinuouses(blockNum, pagesHashes)
	if err != nil {
		return err
	}

	pages := make([][]*big.Int, 0)
	for _, log := range logMemoryPageFactContinuouses {
		// TODO Instead of looping over the logs and submitting an RPC request for each one,
		// we should submit a single batch request. However, the backend
		// doesn't support batch requests at the moment. We should probably construct and send
		// the RPC ourselves.
		tx, _, err := s.l1Reader.TransactionByHash(context.Background(), log.Raw.TxHash)
		if err != nil {
			return err
		}
		// Parse the tx for the calldata
		inputs := make(map[string]interface{})
		// TODO we should make this a receiver on the MemoryPageFactRegistryContract type
		// We shouldn't have to use the Abi
		err = s.memoryPageFactRegistry.Abi.Methods["registerContinuousMemoryPage"].Inputs.UnpackIntoMap(inputs, tx.Data()[4:])
		if err != nil {
			return err
		}
		pages = append(pages, inputs["values"].([]*big.Int))
	}

	update := &types.StateUpdate{
		StateDiff:      *parsePages(pages),
		SequenceNumber: logStateUpdate.BlockNumber.Uint64(),
		NewRoot:        new(felt.Felt).SetBigInt(logStateUpdate.GlobalRoot),
	}

	fmt.Printf("got state update %d\n", update.SequenceNumber)
	return s.commit(update)
}

func (s *L1SyncService) findLogStateTransitionFact(blockNumber uint64, txHash ethcommon.Hash) (*contracts.StarknetLogStateTransitionFact, error) {
	opts := &bind.FilterOpts{
		Start: blockNumber,
		End:   &blockNumber,
	}
	iterator, err := s.starknet.FilterLogStateTransitionFact(opts)
	if err != nil {
		return nil, err
	}
	var logStateTransitionFact *contracts.StarknetLogStateTransitionFact
	for iterator.Next() {
		if iterator.Event.Raw.TxHash == txHash {
			logStateTransitionFact = iterator.Event
		}
	}
	if logStateTransitionFact == nil {
		return nil, errors.New("did not find LogStateTransitionFact for the LogStateUpdate")
	}

	return logStateTransitionFact, nil
}

func (s *L1SyncService) findLogMemoryPagesHashes(startBlockNumber uint64, fact ethcommon.Hash, step uint64) ([][32]byte, uint64, error) {
	start := startBlockNumber - step
	end := startBlockNumber
	for {
		opts := &bind.FilterOpts{
			Start: start,
			End:   &end,
		}

		iterator, err := s.gpsStatementVerifier.FilterLogMemoryPagesHashes(opts)
		if err != nil {
			// TODO deal with rate limiting here
			return nil, 0, err
		}

		for iterator.Next() {
			hash := ethcommon.Hash(iterator.Event.FactHash)
			if hash.Big().Cmp(fact.Big()) == 0 {
				return iterator.Event.PagesHashes, iterator.Event.Raw.BlockNumber, nil
			}
		}

		// We have filtered all of the logs from this contract
		if start == s.gpsStatementVerifier.DeploymentBlock {
			break
		}

		end = start
		if start < step {
			// Avoid underflows in case start-step < 0
			start = 0
		} else {
			start -= step
		}
	}

	return nil, 0, errors.New("could not find LogMemoryPagesHashes for LogStateTransitionFact")
}

func (s *L1SyncService) findLogMemoryPageFactContinuouses(startBlockNumber uint64, pagesHashes [][32]byte) ([]*contracts.MemoryPageFactRegistryLogMemoryPageFactContinuous, error) {
	// When geth can search for logs in reverse chronological order this will be far easier
	// See https://github.com/ethereum/go-ethereum/issues/20593

	numberOfLogsFound := 0
	continuousMemoryPageLogs := make([]*contracts.MemoryPageFactRegistryLogMemoryPageFactContinuous, len(pagesHashes))
	step := uint64(500)
	for start, end := startBlockNumber-step, startBlockNumber; start >= s.memoryPageFactRegistry.DeploymentBlock && numberOfLogsFound != len(pagesHashes); start, end = start-step, start {
		opts := &bind.FilterOpts{
			Start: start,
			End:   &end,
		}
		iterator, err := s.memoryPageFactRegistry.FilterLogMemoryPageFactContinuous(opts)
		if err != nil {
			// TODO deal with rate limiting here
			return nil, err
		}

		tmp := make([]*contracts.MemoryPageFactRegistryLogMemoryPageFactContinuous, 0)
		for iterator.Next() {
			for _, hash := range pagesHashes {
				if ethcommon.BytesToHash(hash[:]).Big().Cmp(iterator.Event.MemoryHash) == 0 {
					tmp = append(tmp, iterator.Event)
				}
			}
		}
		end_ := len(continuousMemoryPageLogs) - numberOfLogsFound
		start_ := end_ - len(tmp)
		copy(continuousMemoryPageLogs[start_:end_], tmp)
		numberOfLogsFound += len(tmp)
	}

	if numberOfLogsFound < len(pagesHashes) {
		return nil, fmt.Errorf("did not find %d continuous memory pages", len(pagesHashes)-numberOfLogsFound)
	}

	return continuousMemoryPageLogs, nil
}

func (s *L1SyncService) getLogStateUpdates(sink chan *contracts.StarknetLogStateUpdate, errChan chan error) {
	l2ChainLength, err := s.manager.L2ChainLength()
	if err != nil {
		if err == db.ErrNotFound {
			l2ChainLength = s.starknet.DeploymentBlock
		} else {
			errChan <- err
			return
		}
	}

	// Subscribe indefinitely
	sub, err := s.starknet.WatchLogStateUpdate(nil, sink)
	if err != nil {
		errChan <- err
		return
	}
	defer sub.Unsubscribe()
	go func() {
		err = <-sub.Err()
		errChan <- err
	}()

	chainHeader, err := s.l1Reader.HeaderByNumber(context.Background(), nil)
	if err != nil {
		errChan <- err
		return
	}
	l1ChainLength := chainHeader.Number.Uint64()

	// TODO more clever error handling and stepping
	step := uint64(500)
	for start, end := l2ChainLength, l2ChainLength+step; end < l1ChainLength; start, end = end, end+step {
		opts := &bind.FilterOpts{
			Start: start,
			End:   &end,
			// TODO set a context and timeout
		}

		iterator, err := s.starknet.FilterLogStateUpdate(opts)
		if err != nil {
			// TODO handle rate limiting here
			errChan <- err
			return
		}

		for iterator.Next() {
			sink <- iterator.Event
		}

		if err = iterator.Close(); err != nil {
			errChan <- err
			return
		}

		if end+step > l1ChainLength {
			step = l1ChainLength - 1
		}
	}
}

// parsePages converts an array of memory pages into a state diff that
// can be used to update the local state.
// TODO clean up this function
func parsePages(pages [][]*big.Int) *types.StateDiff {
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

	storageDiff := make(map[string][]types.MemoryCell, 0)

	// Iterate over all the contracts that had been updated and collect the needed information
	for i := int64(0); i < numContractsUpdate; i++ {
		// Parse the Address of the contract
		address := new(felt.Felt).SetBigInt(pagesFlatter[0])
		pagesFlatter = pagesFlatter[1:]

		// Parse the number storage updates
		numStorageUpdates := pagesFlatter[0].Int64()
		pagesFlatter = pagesFlatter[1:]

		kvs := make([]types.MemoryCell, 0)
		for k := int64(0); k < numStorageUpdates; k++ {
			kvs = append(kvs, types.MemoryCell{
				Address: new(felt.Felt).SetBigInt(pagesFlatter[0]),
				Value:   new(felt.Felt).SetBigInt(pagesFlatter[1]),
			})
			pagesFlatter = pagesFlatter[2:]
		}
		storageDiff[address.String()] = kvs
	}

	return &types.StateDiff{
		DeployedContracts: deployedContracts,
		StorageDiff:       storageDiff,
	}
}

func (s *L1SyncService) commit(newUpdate *types.StateUpdate) error {
	heap.Push(s.queue, *newUpdate)

	// TODO we can do some creative things here, like batching the state updates
	// during the sync to mitigate I/O congestion
	// Also: it may be better to use a channel than a mutex, especially once database performance improves
	for ; s.queue.Len() > 0 && s.queue.Peek().(*types.StateUpdate).SequenceNumber == s.nextBlock; s.nextBlock++ {
		start := time.Now()
		update := heap.Pop(s.queue).(*item).value

		for _, deployedContract := range update.StateDiff.DeployedContracts {
			// The Contract Code is nil since it isn't on L1. It will be set in the L2 sync.
			// TODO we probably should not be storing the code with the hash at all.
			err := s.state.SetContract(deployedContract.Address, deployedContract.Hash)
			if err != nil {
				return err
			}
		}

		for contractAddress, memoryCells := range update.StateDiff.StorageDiff {
			for _, cell := range memoryCells {
				err := s.state.SetSlot(new(felt.Felt).SetString(contractAddress), cell.Address, cell.Value)
				if err != nil {
					return err
				}
			}
		}

		if s.state.Root().Cmp(update.NewRoot) == 0 {
			fmt.Printf("synced %d in %f seconds\n", update.SequenceNumber, time.Since(start).Seconds())
		} else {
			// This should never happen, only here for debugging purposes.
			panic("state roots not equal for block " + string(rune(update.SequenceNumber)))
		}
	}

	return nil
}

package sync

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/internal/db"
	syncdb "github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/sync/contracts"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type l1Reader interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*ethtypes.Header, error)
	TransactionByHash(ctx context.Context, txHash ethcommon.Hash) (tx *ethtypes.Transaction, isPending bool, err error)
}

type L1Backend interface {
	bind.ContractBackend
	l1Reader
}

// This is data that technically belongs in the bindgen code, but isn't provided so we manually input it ourselves.
type contractsMetadata struct {
	starknetDeploymentBlock               uint64
	gpsStatementVerifierDeploymentBlock   uint64
	memoryPageFactRegistryDeploymentBlock uint64
	memoryPageFactRegistryAbi             *abi.ABI
}

type L1SyncService struct {
	// L1 StarkNet Contract bindings
	starknet               *contracts.Starknet
	gpsStatementVerifier   *contracts.GpsStatementVerifier
	memoryPageFactRegistry *contracts.MemoryPageFactRegistry

	contractsMetadata
	manager  *syncdb.Manager
	l1Reader l1Reader

	state state.State

	// An in-memory priority queue of the state updates not yet
	// committed to the database.
	queue StateUpdateQueue
}

func NewL1SyncService(starknetAddress, gpsStatementVerifierAddress, memoryPageFactRegistryAddress ethcommon.Address, deploymentBlock uint64, backend L1Backend) (*L1SyncService, error) {
	starknet, err := contracts.NewStarknet(starknetAddress, backend)
	if err != nil {
		return nil, err
	}
	gpsStatementVerifier, err := contracts.NewGpsStatementVerifier(gpsStatementVerifierAddress, nil)
	if err != nil {
		return nil, err
	}
	memoryPageFactRegistry, err := contracts.NewMemoryPageFactRegistry(memoryPageFactRegistryAddress, nil)
	if err != nil {
		return nil, err
	}

	// TODO initialize all fields here
	return &L1SyncService{
		starknet:               starknet,
		gpsStatementVerifier:   gpsStatementVerifier,
		memoryPageFactRegistry: memoryPageFactRegistry,
		contractsMetadata: contractsMetadata{
			starknetDeploymentBlock: deploymentBlock,
		},
	}, nil
}

// TODO double-check addresses against pathfinder (are they using the proxies or the actual?)

func NewMainnetL1SyncService(backend L1Backend) (*L1SyncService, error) {
	starknetAddress := ethcommon.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4")
	gpsStatementVerifierAddress := ethcommon.HexToAddress("0xa739b175325cca7b71fcb51c3032935ef7ac338f")
	memoryPageFactRegistryAddress := ethcommon.HexToAddress("0x96375087b2f6efc59e5e0dd5111b4d090ebfdd8b")
	return NewL1SyncService(starknetAddress, gpsStatementVerifierAddress, memoryPageFactRegistryAddress, 13617000, backend)
}

func NewGoerliL1SyncService(backend L1Backend) (*L1SyncService, error) {
	starknetAddress := ethcommon.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e")
	gpsStatementVerifierAddress := ethcommon.HexToAddress("0x5ef3c980bf970fce5bbc217835743ea9f0388f4f")
	memoryPageFactRegistryAddress := ethcommon.HexToAddress("0x743789ff2ff82bfb907009c9911a7da636d34fa7")
	return NewL1SyncService(starknetAddress, gpsStatementVerifierAddress, memoryPageFactRegistryAddress, 5840000, backend)
}

func (s *L1SyncService) Run(errChan chan error) {
	logStateUpdates := make(chan *contracts.StarknetLogStateUpdate)
	go s.getLogStateUpdates(logStateUpdates, errChan)

	for logStateUpdate := range logStateUpdates {
		go s.updateState(logStateUpdate, errChan)
	}
}

func (s *L1SyncService) updateState(logStateUpdate *contracts.StarknetLogStateUpdate, errChan chan error) {
	logStateTransitionFact, err := s.findLogStateTransitionFact(logStateUpdate)
	if err != nil {
		errChan <- err
		return
	}

	logMemoryPagesHashes, err := s.findLogMemoryPagesHashes(logStateTransitionFact)
	if err != nil {
		errChan <- err
		return
	}

	logMemoryPageFactContinuouses, err := s.findLogMemoryPageFactContinuouses(logMemoryPagesHashes)
	if err != nil {
		errChan <- err
		return
	}

	pages := make([][]*big.Int, 0)
	for _, log := range logMemoryPageFactContinuouses {
		// TODO Instead of looping over the logs and submitting an RPC request for each one,
		// we should submit a single batch request. However, the backend
		// doesn't support batch requests at the moment. We should probably construct and send
		// the RPC ourselves.
		tx, _, err := s.l1Reader.TransactionByHash(context.Background(), log.Raw.TxHash)
		if err != nil {
			errChan <- err
			return
		}
		// Parse the tx for the calldata
		inputs := make(map[string]interface{})
		err = s.memoryPageFactRegistryAbi.Methods["registerContinuousMemoryPage"].Inputs.UnpackIntoMap(inputs, tx.Data()[4:])
		if err != nil {
			errChan <- err
			return
		}
		pages = append(pages, inputs["values"].([]*big.Int))
	}

	stateUpdate := &types.StateUpdate{
		StateDiff:      *parsePages(pages),
		SequenceNumber: logStateUpdate.BlockNumber.Uint64(),
		NewRoot:        new(felt.Felt).SetBigInt(logStateUpdate.GlobalRoot),
	}

	if err := s.commit(stateUpdate); err != nil {
		errChan <- err
		return
	}
}

func (s *L1SyncService) findLogStateTransitionFact(logStateUpdate *contracts.StarknetLogStateUpdate) (*contracts.StarknetLogStateTransitionFact, error) {
	opts := &bind.FilterOpts{
		Start: logStateUpdate.Raw.BlockNumber,
		End:   &logStateUpdate.Raw.BlockNumber,
	}
	iterator, err := s.starknet.FilterLogStateTransitionFact(opts)
	if err != nil {
		return nil, err
	}
	var logStateTransitionFact *contracts.StarknetLogStateTransitionFact
	for iterator.Next() {
		if iterator.Event.Raw.TxHash == logStateUpdate.Raw.TxHash {
			logStateTransitionFact = iterator.Event
		}
	}
	if logStateTransitionFact == nil {
		return nil, errors.New("did not find LogStateTransitionFact for the LogStateUpdate")
	}

	return logStateTransitionFact, nil
}

func (s *L1SyncService) findLogMemoryPagesHashes(logStateTransitionFact *contracts.StarknetLogStateTransitionFact) (*contracts.GpsStatementVerifierLogMemoryPagesHashes, error) {
	step := uint64(500)
	for start, end := logStateTransitionFact.Raw.BlockNumber-step, logStateTransitionFact.Raw.BlockNumber; start < s.gpsStatementVerifierDeploymentBlock; start, end = start-step, start {
		opts := &bind.FilterOpts{
			Start: start,
			End:   &end,
		}
		iterator, err := s.gpsStatementVerifier.FilterLogMemoryPagesHashes(opts)
		if err != nil {
			return nil, err
		}

		fact := ethcommon.BytesToHash(logStateTransitionFact.StateTransitionFact[:])
		for iterator.Next() {
			if ethcommon.BytesToHash(iterator.Event.FactHash[:]).Big().Cmp(fact.Big()) == 0 {
				return iterator.Event, nil
			}
		}
	}

	return nil, errors.New("could not find LogMemoryPagesHashes for LogStateTransitionFact")
}

func (s *L1SyncService) findLogMemoryPageFactContinuouses(logMemoryPagesHashes *contracts.GpsStatementVerifierLogMemoryPagesHashes) ([]contracts.MemoryPageFactRegistryLogMemoryPageFactContinuous, error) {
	// When geth can search for logs in reverse chronological order this will be far easier
	// See https://github.com/ethereum/go-ethereum/issues/20593 for somewhat related issue

	pagesHashes := logMemoryPagesHashes.PagesHashes

	numberOfLogsFound := 0
	continuousMemoryPageLogs := make([]contracts.MemoryPageFactRegistryLogMemoryPageFactContinuous, len(pagesHashes))
	step := uint64(500)
	for start, end := logMemoryPagesHashes.Raw.BlockNumber-step, logMemoryPagesHashes.Raw.BlockNumber; start < s.memoryPageFactRegistryDeploymentBlock; start, end = start-step, start {
		opts := &bind.FilterOpts{
			Start: start,
			End:   &end,
		}
		iterator, err := s.memoryPageFactRegistry.FilterLogMemoryPageFactContinuous(opts)
		if err != nil {
			return nil, err
		}

		for iterator.Next() {
			for _, hash := range pagesHashes {
				if ethcommon.BytesToHash(hash[:]).Big().Cmp(iterator.Event.Raw.TxHash.Big()) == 0 {
					continuousMemoryPageLogs[len(continuousMemoryPageLogs)-numberOfLogsFound] = *iterator.Event
					numberOfLogsFound++
				}
			}
		}

		if numberOfLogsFound == len(pagesHashes) {
			break
		}
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
			l2ChainLength = s.starknetDeploymentBlock
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
			errChan <- err // TODO can we recover
			return
		}

		for iterator.Next() {
			sink <- iterator.Event
		}

		if err = iterator.Close(); err != nil {
			errChan <- err // TODO can we recover
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

func (s *L1SyncService) commit(update *types.StateUpdate) error {
	heap.Push(&s.queue, update)

	// TODO this is buggy - we need to get next update here

	// TODO we can do some creative things here, like batching the state updates
	// during the sync to mitigate I/O congestion
	// To do this, we need a better way to track sync progress relative to the
	// head of the L2 chain
	for _, deployedContract := range update.StateDiff.DeployedContracts {
		// The Contract Code is nil since it isn't on L1. It will be set in the L2 sync.
		// TODO we should not be storing the code with the hash at all.
		err := s.state.SetContract(deployedContract.Address, deployedContract.Hash, nil)
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
	return nil
}

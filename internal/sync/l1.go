package sync

import (
	"errors"

	"github.com/NethermindEth/juno/internal/db"
	syncdb "github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/sync/contracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type L1SyncService struct {
	// L1 StarkNet Contracts
	starknet               *contracts.Starknet
	gpsStatementVerifier   *contracts.GpsStatementVerifier
	memoryPageFactRegistry *contracts.MemoryPageFactRegistry

	manager                               syncdb.Manager
	starknetDeploymentBlock               uint64
	gpsStatementVerifierDeploymentBlock   uint64
	memoryPageFactRegistryDeploymentBlock uint64
	l1ChainHeadChan                       chan ethtypes.Header
}

func NewL1SyncService(starknetAddress, gpsStatementVerifierAddress, memoryPageFactRegistryAddress ethcommon.Address, deploymentBlock uint64, backend bind.ContractBackend) (*L1SyncService, error) {
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

	return &L1SyncService{
		starknet:                starknet,
		gpsStatementVerifier:    gpsStatementVerifier,
		memoryPageFactRegistry:  memoryPageFactRegistry,
		starknetDeploymentBlock: deploymentBlock,
	}, nil
}

// TODO double-check addresses

func NewMainnetL1SyncService(backend bind.ContractBackend) (*L1SyncService, error) {
	starknetAddress := ethcommon.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4")
	gpsStatementVerifierAddress := ethcommon.HexToAddress("0x47312450B3Ac8b5b8e247a6bB6d523e7605bDb60")
	memoryPageFactRegistryAddress := ethcommon.HexToAddress("0x96375087b2f6efc59e5e0dd5111b4d090ebfdd8b")
	return NewL1SyncService(starknetAddress, gpsStatementVerifierAddress, memoryPageFactRegistryAddress, 13617000, backend)
}

func NewGoerliL1SyncService(backend bind.ContractBackend) (*L1SyncService, error) {
	starknetAddress := ethcommon.HexToAddress("") // TODO
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

	// TODO
	// - send a batch RPC request for all of the registerContinuousMemoryPage transactions.
	// - parse into pages
	// - send the state diff to the manager
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
		return nil, errors.New("Did not find LogStateTransitionFact for the LogStateUpdate")
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

	return nil, errors.New("Could not find LogMemoryPagesHashes for LogStateTransitionFact")
}

func (s *L1SyncService) findLogMemoryPageFactContinuouses(logMemoryPagesHashes *contracts.GpsStatementVerifierLogMemoryPagesHashes) ([]ethcommon.Hash, error) {
	return nil, nil
	// TODO We need to search backwards for every LogMemoryPageFactContinuous that is present in the MemoryPagesHashes

	//pagesHashes := logMemoryPagesHashes.PagesHashes
	//step := uint64(500)
	//for start, end := logMemoryPagesHashes.Raw.BlockNumber - step, logMemoryPagesHashes.Raw.BlockNumber; start < s.memoryPageFactRegistryDeploymentBlock; start, end = start - step, start {
	//	opts := &bind.FilterOpts{
	//		Start: start,
	//		End: &end,
	//	}
	//	iterator, err := s.memoryPageFactRegistry.FilterLogMemoryPageFactContinuous(opts)
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//	registerContinuousMemoryPageTxHashes := make([]ethcommon.Hash, len(pagesHashes))
	//	for i := 0; iterator.Next(); i++ {
	//		registerContinuousMemoryPageTxHashes[len(registerContinuousMemoryPageTxHashes) - i] = iterator.Event.Raw.TxHash
	//	}
	//}
	//return nil, nil
}

func (s *L1SyncService) getLogStateUpdates(sink chan *contracts.StarknetLogStateUpdate, errChan chan error) {
	l1ChainLength, err := s.manager.L1ChainLength()
	if err != nil {
		if err == db.ErrNotFound {
			l1ChainLength = s.starknetDeploymentBlock
		} else {
			errChan <- err
			return
		}
	}

	// TODO implement binary search on step
	step := uint64(500)
	for start, end := l1ChainLength, l1ChainLength+step; ; start, end = end, end+step {
		// Poll until we reach the head of the L1 Chain
		if end > (<-s.l1ChainHeadChan).Number.Uint64() {
			break
		}
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
	}

	// Subscribe indefinitely
	sub, err := s.starknet.WatchLogStateUpdate(nil, sink)
	if err != nil {
		errChan <- err
		return
	}
	defer sub.Unsubscribe()
	err = <-sub.Err()
	errChan <- err
}

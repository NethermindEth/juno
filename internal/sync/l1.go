package sync

import (
	"container/heap"
	"context"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/internal/db"
	syncdb "github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/sync/contracts"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type ContractConfig struct {
	address         ethcommon.Address
	deploymentBlock uint64
}

type l1Reader interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*ethtypes.Header, error)
}

type L1Backend interface {
	ethereum.LogFilterer
	l1Reader
}

type L1SyncService struct {
	starknet contracts.Contract

	manager  *syncdb.Manager
	l1Reader l1Reader

	state     state.State
	nextBlock uint64

	// An in-memory priority queue of the state updates not yet
	// committed to the database.
	queue *LogStateUpdateQueue
}

func NewL1SyncService(starknetConfig *ContractConfig, backend L1Backend, syncManager *syncdb.Manager, stateManager state.StateManager) (*L1SyncService, error) {
	starknet, err := contracts.NewStarknet(starknetConfig.address, starknetConfig.deploymentBlock, backend)
	if err != nil {
		return nil, err
	}

	return &L1SyncService{
		starknet:  starknet,
		l1Reader:  backend,
		manager:   syncManager,
		state:     state.New(stateManager, new(felt.Felt)),
		queue:     NewStateUpdateQueue(),
		nextBlock: 0, // TODO we should get this from the sync manager
	}, nil
}

// TODO get more accurate deployment blocks

func NewMainnetL1SyncService(backend L1Backend, syncManager *syncdb.Manager, stateManager state.StateManager) (*L1SyncService, error) {
	starknetConfig := &ContractConfig{
		address:         ethcommon.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		deploymentBlock: 13617000,
	}
	return NewL1SyncService(starknetConfig, backend, syncManager, stateManager)
}

func NewGoerliL1SyncService(backend L1Backend, syncManager *syncdb.Manager, stateManager state.StateManager) (*L1SyncService, error) {
	starknetConfig := &ContractConfig{
		address:         ethcommon.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e"),
		deploymentBlock: 5840000,
	}
	return NewL1SyncService(starknetConfig, backend, syncManager, stateManager)
}

func (s *L1SyncService) Run(errChan chan error) {
	logStateUpdates := make(chan *contracts.LogStateUpdate)
	go s.getLogStateUpdates(logStateUpdates, errChan)

	for logStateUpdate := range logStateUpdates {
		go func(logStateUpdate *contracts.LogStateUpdate) {
			if err := s.commit(logStateUpdate); err != nil {
				errChan <- err
			}
		}(logStateUpdate)
	}
}

func (s *L1SyncService) Close() error {
	s.manager.Close()
	return nil
}

func (s *L1SyncService) getLogStateUpdates(sink chan *contracts.LogStateUpdate, errChan chan error) {
	l2ChainLength, err := s.manager.L2ChainLength()
	if err != nil {
		if err == db.ErrNotFound {
			l2ChainLength = s.starknet.DeploymentBlock()
		} else {
			errChan <- err
			return
		}
	}

	// Subscribe indefinitely
	iterator, err := s.starknet.SubscribeFilterLogs(context.Background(), contracts.LogStateUpdateName)
	if err != nil {
		errChan <- err
		return
	}
	go func() {
		if err := iterator.Error(); err != nil {
			errChan <- err
		}

		for iterator.Next() {
			logStateUpdate := new(contracts.LogStateUpdate)
			err := s.starknet.ParseLog(logStateUpdate, iterator.Log(), contracts.LogStateUpdateName)
			if err != nil {
				errChan <- err
			}
			sink <- logStateUpdate
		}
	}()

	chainHeader, err := s.l1Reader.HeaderByNumber(context.Background(), nil)
	if err != nil {
		errChan <- err
		return
	}
	l1ChainLength := chainHeader.Number.Uint64()

	// TODO move rate limiting logic to contracts.contract.FilterLogs
	step := uint64(500)
	for start, end := l2ChainLength, l2ChainLength+step; end < l1ChainLength; start, end = end, end+step {
		iterator, err := s.starknet.FilterLogs(context.Background(), start, end, contracts.LogStateUpdateName)
		if err != nil {
			errChan <- err
			return
		}

		for iterator.Next() {
			logStateUpdate := new(contracts.LogStateUpdate)
			err := s.starknet.ParseLog(logStateUpdate, iterator.Log(), contracts.LogStateUpdateName)
			if err != nil {
				errChan <- err
			}
			sink <- logStateUpdate
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

func (s *L1SyncService) commit(newUpdate *contracts.LogStateUpdate) error {
	heap.Push(s.queue, *newUpdate)

	for ; s.queue.Len() > 0 && s.queue.Peek().(contracts.LogStateUpdate).BlockNumber.Uint64() == s.nextBlock; s.nextBlock++ {
		update := heap.Pop(s.queue).(*item).value
		// TODO handle reorgs here
		fmt.Printf("found update for block %d\n", update.BlockNumber)
	}

	return nil
}

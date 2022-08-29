package health

import (
	"github.com/NethermindEth/juno/internal/cairovm"
	"go.uber.org/zap"

	sync2 "github.com/NethermindEth/juno/internal/sync"

	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/transaction"

	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/state"
)

type Rpc struct {
	stateManager state.StateManager
	blockManager *block.Manager
	txnManager   *transaction.Manager
	synchronizer *sync2.Synchronizer
	vm           *cairovm.VirtualMachine
	logger       *zap.SugaredLogger
}

func New(stateManager state.StateManager, blockManager *block.Manager, txnManager *transaction.Manager,
	synchronizer *sync2.Synchronizer, vm *cairovm.VirtualMachine,
) *Rpc {
	return &Rpc{
		stateManager: stateManager,
		blockManager: blockManager,
		txnManager:   txnManager,
		synchronizer: synchronizer,
		vm:           vm,
		logger:       Logger.Named("RPC"),
	}
}

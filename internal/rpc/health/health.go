package health

import (
	"github.com/NethermindEth/juno/internal/cairovm"
	"go.uber.org/zap"

	sync2 "github.com/NethermindEth/juno/internal/sync"

	. "github.com/NethermindEth/juno/internal/log"
)

type Rpc struct {
	synchronizer *sync2.Synchronizer
	vm           *cairovm.VirtualMachine
	logger       *zap.SugaredLogger
}

// New returns a new health rpc service.
// notest
func New(synchronizer *sync2.Synchronizer, vm *cairovm.VirtualMachine) *Rpc {
	return &Rpc{
		synchronizer: synchronizer,
		vm:           vm,
		logger:       Logger.Named("Health Check RPC"),
	}
}

// NodeStatus returns the current node status.
// notest
func (r *Rpc) NodeStatus() (any, error) {
	if r.synchronizer.Running && r.vm.Running() {
		return Status{
			Status:        "Healthy",
			Message:       "Node running",
			SyncingStatus: r.synchronizer.Status(),
		}, nil
	}
	if !r.synchronizer.Running {
		return nil, NodeNotSyncing
	}
	if !r.vm.Running() {
		return nil, VMNotRunning
	}
	return nil, UnHealthy
}

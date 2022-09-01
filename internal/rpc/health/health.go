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

type SyncStatus struct {
	CurrentBlockNumber uint64 `json:"current_block_number"`
	HighestBlockNumber uint64 `json:"highest_block_number"`
}

type Status struct {
	Status        string      `json:"status"`
	Message       string      `json:"message"`
	SyncingStatus *SyncStatus `json:"syncing_status"`
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
		status := r.synchronizer.Status()
		return Status{
			Status:  "Healthy",
			Message: "Node running",
			SyncingStatus: &SyncStatus{
				CurrentBlockNumber: status.CurrentBlockNumber,
				HighestBlockNumber: status.HighestBlockNumber,
			},
		}, nil
	}
	if !r.synchronizer.Running {
		return nil, ErrorNodeNotSyncing
	}
	if !r.vm.Running() {
		return nil, ErrorVMNotRunning
	}
	return nil, ErrorUnHealthy
}

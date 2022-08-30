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
	synchronizer *sync2.Synchronizer
	vm           *cairovm.VirtualMachine
	logger       *zap.SugaredLogger
}

func New(stateManager state.StateManager, blockManager *block.Manager, txnManager *transaction.Manager,
	synchronizer *sync2.Synchronizer, vm *cairovm.VirtualMachine,
) *Rpc {
	return &Rpc{
		synchronizer: synchronizer,
		vm:           vm,
		logger:       Logger.Named("Health Check RPC"),
	}
}

// // Request
//curl localhost:8545/health
//
//// Example of response for Unhealthy node
//{"status":"Unhealthy","totalDuration":"00:00:00.0015582","entries":{"node-health":{"data":{},"description":"The node has 0 peers connected","duration":"00:00:00.0003881","status":"Unhealthy","tags":[]}}}
//
//// Example of response for Healthy node
//{
//"status":"Healthy",
//"totalDuration":"00:00:00.0015582",
//"entries":{
//	"node-health":{
//		"data":{},
//		"description":"The node is now fully synced with a network, number of peers: 99",
//		"duration":"00:00:00.0003881","status":"Healthy","tags":[]}}}

func (r *Rpc) NodeStatus() (any, error) {

	return nil, nil
}

func (r *Rpc) Available() (any, error) {
	return r.synchronizer.Running, nil
}

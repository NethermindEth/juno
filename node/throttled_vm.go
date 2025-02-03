package node

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

var _ vm.VM = (*ThrottledVM)(nil)

type ThrottledVM struct {
	*utils.Throttler[vm.VM]
}

func NewThrottledVM(res vm.VM, concurrenyBudget uint, maxQueueLen int32) *ThrottledVM {
	return &ThrottledVM{
		Throttler: utils.NewThrottler(concurrenyBudget, &res).WithMaxQueueLen(maxQueueLen),
	}
}

func (tvm *ThrottledVM) Call(callInfo *vm.CallInfo, blockInfo *vm.BlockInfo, state core.StateReader,
	network *utils.Network, maxSteps uint64,
) ([]*felt.Felt, error) {
	var ret []*felt.Felt
	return ret, tvm.Do(func(vm *vm.VM) error {
		var err error
		ret, err = (*vm).Call(callInfo, blockInfo, state, network, maxSteps)
		return err
	})
}

func (tvm *ThrottledVM) Execute(txns []core.Transaction, declaredClasses []core.Class, paidFeesOnL1 []*felt.Felt,
	blockInfo *vm.BlockInfo, state core.StateReader, network *utils.Network, skipChargeFee, skipValidate, errOnRevert bool,
) (vm.ExecutionResults, error) {
	var executionResult vm.ExecutionResults
	return executionResult, tvm.Do(func(vm *vm.VM) error {
		var err error
		executionResult, err = (*vm).Execute(txns, declaredClasses, paidFeesOnL1, blockInfo, state, network,
			skipChargeFee, skipValidate, errOnRevert)
		return err
	})
}

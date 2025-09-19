package node

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/types/felt"
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
	network *utils.Network, maxSteps uint64, sierraVersion string, errStack, returnStateDiff bool,
) (vm.CallResult, error) {
	ret := vm.CallResult{}
	return ret, tvm.Do(func(vm *vm.VM) error {
		var err error
		ret, err = (*vm).Call(callInfo, blockInfo, state, network, maxSteps, sierraVersion, errStack, returnStateDiff)
		return err
	})
}

func (tvm *ThrottledVM) Execute(txns []core.Transaction, declaredClasses []core.Class, paidFeesOnL1 []*felt.Felt,
	blockInfo *vm.BlockInfo, state core.StateReader, network *utils.Network, skipChargeFee, skipValidate, errOnRevert, errStack,
	allowBinarySearch bool,
) (vm.ExecutionResults, error) {
	var executionResult vm.ExecutionResults
	return executionResult, tvm.Do(func(vm *vm.VM) error {
		var err error
		executionResult, err = (*vm).Execute(txns, declaredClasses, paidFeesOnL1, blockInfo, state, network,
			skipChargeFee, skipValidate, errOnRevert, errStack, allowBinarySearch)
		return err
	})
}

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

func (tvm *ThrottledVM) Execute(txns []core.Transaction, declaredClasses []core.Class, paidFeesOnL1 []*felt.Felt, //nolint:gocritic
	blockInfo *vm.BlockInfo, state core.StateReader, network *utils.Network, skipChargeFee, skipValidate, errOnRevert bool,
) ([]*felt.Felt, []core.DataAvailability, []core.GasConsumed, []vm.TransactionTrace, uint64, error) {
	var ret []*felt.Felt
	var traces []vm.TransactionTrace
	var daGas []core.DataAvailability
	var gasConsumed []core.GasConsumed
	var numSteps uint64
	return ret, daGas, gasConsumed, traces, numSteps, tvm.Do(func(vm *vm.VM) error {
		var err error
		ret, daGas, gasConsumed, traces, numSteps, err = (*vm).Execute(txns, declaredClasses, paidFeesOnL1, blockInfo, state, network,
			skipChargeFee, skipValidate, errOnRevert)
		return err
	})
}

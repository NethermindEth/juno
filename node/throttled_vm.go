package node

import (
	"context"

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
		Throttler: utils.NewThrottler[vm.VM](maxQueueLen, concurrenyBudget, &res),
	}
}

func (tvm *ThrottledVM) Call(ctx context.Context, callInfo *vm.CallInfo, blockInfo *vm.BlockInfo, state core.StateReader,
	network *utils.Network, maxSteps uint64, useBlobData bool,
) ([]*felt.Felt, error) {
	var ret []*felt.Felt
	return ret, tvm.Do(ctx, func(vm *vm.VM) error {
		var err error
		ret, err = (*vm).Call(ctx, callInfo, blockInfo, state, network, maxSteps, useBlobData)
		return err
	})
}

func (tvm *ThrottledVM) Execute(ctx context.Context, txns []core.Transaction, declaredClasses []core.Class, paidFeesOnL1 []*felt.Felt,
	blockInfo *vm.BlockInfo, state core.StateReader, network *utils.Network, skipChargeFee, skipValidate, errOnRevert,
	useBlobData bool,
) ([]*felt.Felt, []*felt.Felt, []vm.TransactionTrace, uint64, error) {
	var (
		ret             []*felt.Felt
		traces          []vm.TransactionTrace
		dataGasConsumed []*felt.Felt

		numSteps uint64
	)
	return ret, dataGasConsumed, traces, numSteps, tvm.Do(ctx, func(vm *vm.VM) error {
		var err error
		ret, dataGasConsumed, traces, numSteps, err = (*vm).Execute(ctx, txns, declaredClasses, paidFeesOnL1, blockInfo, state, network,
			skipChargeFee, skipValidate, errOnRevert, useBlobData)
		return err
	})
}

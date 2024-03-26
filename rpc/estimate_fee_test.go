package rpc_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEstimateMessageFee(t *testing.T) {
	t.Skip()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&utils.Mainnet).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)

	handler := rpc.New(mockReader, nil, mockVM, "", utils.NewNopZapLogger())
	msg := rpc.MsgFromL1{
		From:     common.HexToAddress("0xDEADBEEF"),
		To:       *new(felt.Felt).SetUint64(1337),
		Payload:  []felt.Felt{*new(felt.Felt).SetUint64(1), *new(felt.Felt).SetUint64(2)},
		Selector: *new(felt.Felt).SetUint64(44),
	}

	t.Run("block not found", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)
		_, err := handler.EstimateMessageFeeV0_6(msg, rpc.BlockID{Latest: true})
		require.Equal(t, rpc.ErrBlockNotFound, err)
	})

	latestHeader := &core.Header{
		Number:    9,
		Timestamp: 456,
		GasPrice:  new(felt.Felt).SetUint64(42),
	}
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
	mockReader.EXPECT().HeadsHeader().Return(latestHeader, nil)

	expectedGasConsumed := new(felt.Felt).SetUint64(37)
	mockVM.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), &vm.BlockInfo{
		Header: latestHeader,
	}, gomock.Any(), &utils.Mainnet, gomock.Any(), false, true, false).DoAndReturn(
		func(txns []core.Transaction, declaredClasses []core.Class, paidFeesOnL1 []*felt.Felt, blockInfo *vm.BlockInfo,
			state core.StateReader, network *utils.Network, skipChargeFee, skipValidate, errOnRevert bool,
		) ([]*felt.Felt, []*felt.Felt, []vm.TransactionTrace, error) {
			require.Len(t, txns, 1)
			assert.NotNil(t, txns[0].(*core.L1HandlerTransaction))

			assert.Empty(t, declaredClasses)
			assert.Len(t, paidFeesOnL1, 1)

			actualFee := new(felt.Felt).Mul(expectedGasConsumed, blockInfo.Header.GasPrice)
			return []*felt.Felt{actualFee}, []*felt.Felt{&felt.Zero}, []vm.TransactionTrace{{
				StateDiff: &vm.StateDiff{
					StorageDiffs:              []vm.StorageDiff{},
					Nonces:                    []vm.Nonce{},
					DeployedContracts:         []vm.DeployedContract{},
					DeprecatedDeclaredClasses: []*felt.Felt{},
					DeclaredClasses:           []vm.DeclaredClass{},
					ReplacedClasses:           []vm.ReplacedClass{},
				},
			}}, nil
		},
	)

	estimateFee, err := handler.EstimateMessageFeeV0_6(msg, rpc.BlockID{Latest: true})
	require.Nil(t, err)
	feeUnit := rpc.WEI
	require.Equal(t, rpc.FeeEstimate{
		GasConsumed: expectedGasConsumed,
		GasPrice:    latestHeader.GasPrice,
		OverallFee:  new(felt.Felt).Mul(expectedGasConsumed, latestHeader.GasPrice),
		Unit:        &feeUnit,
	}, *estimateFee)
}

func TestEstimateFee(t *testing.T) {
	t.Skip()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	network := utils.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&network).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, mockVM, "", log)

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil).AnyTimes()

	blockInfo := vm.BlockInfo{Header: &core.Header{}}
	t.Run("ok with zero values", func(t *testing.T) {
		mockVM.EXPECT().Execute(nil, nil, []*felt.Felt{}, &blockInfo, mockState, &network, true, true, false, false).
			Return([]*felt.Felt{}, []vm.TransactionTrace{}, nil)

		_, err := handler.EstimateFee([]rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{}, rpc.BlockID{Latest: true})
		require.Nil(t, err)
	})

	t.Run("ok with zero values, skip validate", func(t *testing.T) {
		mockVM.EXPECT().Execute(nil, nil, []*felt.Felt{}, &blockInfo, mockState, &network, true, true, false, false).
			Return([]*felt.Felt{}, []vm.TransactionTrace{}, nil)

		_, err := handler.EstimateFee([]rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag}, rpc.BlockID{Latest: true})
		require.Nil(t, err)
	})

	t.Run("transaction execution error", func(t *testing.T) {
		mockVM.EXPECT().Execute(nil, nil, []*felt.Felt{}, &blockInfo, mockState, &network, true, true, false, false).
			Return(nil, nil, vm.TransactionExecutionError{
				Index: 44,
				Cause: errors.New("oops"),
			})

		_, err := handler.EstimateFee([]rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag}, rpc.BlockID{Latest: true})
		require.Equal(t, rpc.ErrTransactionExecutionError.CloneWithData(rpc.TransactionExecutionErrorData{
			TransactionIndex: 44,
			ExecutionError:   "oops",
		}), err)

		mockVM.EXPECT().Execute(nil, nil, []*felt.Felt{}, &blockInfo, mockState, &network, false, true, true, false).
			Return(nil, nil, vm.TransactionExecutionError{
				Index: 44,
				Cause: errors.New("oops"),
			})
	})
}

func assertEqualCairo0Class(t *testing.T, cairo0Class *core.Cairo0Class, class *rpc.Class) {
	assert.Equal(t, cairo0Class.Program, class.Program)
	assert.Equal(t, cairo0Class.Abi, class.Abi.(json.RawMessage))

	require.Equal(t, len(cairo0Class.L1Handlers), len(class.EntryPoints.L1Handler))
	for idx := range cairo0Class.L1Handlers {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Index)
		assert.Equal(t, cairo0Class.L1Handlers[idx].Offset, class.EntryPoints.L1Handler[idx].Offset)
		assert.Equal(t, cairo0Class.L1Handlers[idx].Selector, class.EntryPoints.L1Handler[idx].Selector)
	}

	require.Equal(t, len(cairo0Class.Constructors), len(class.EntryPoints.Constructor))
	for idx := range cairo0Class.Constructors {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Index)
		assert.Equal(t, cairo0Class.Constructors[idx].Offset, class.EntryPoints.Constructor[idx].Offset)
		assert.Equal(t, cairo0Class.Constructors[idx].Selector, class.EntryPoints.Constructor[idx].Selector)
	}

	require.Equal(t, len(cairo0Class.Externals), len(class.EntryPoints.External))
	for idx := range cairo0Class.Externals {
		assert.Nil(t, class.EntryPoints.External[idx].Index)
		assert.Equal(t, cairo0Class.Externals[idx].Offset, class.EntryPoints.External[idx].Offset)
		assert.Equal(t, cairo0Class.Externals[idx].Selector, class.EntryPoints.External[idx].Selector)
	}
}

func assertEqualCairo1Class(t *testing.T, cairo1Class *core.Cairo1Class, class *rpc.Class) {
	assert.Equal(t, cairo1Class.Program, class.SierraProgram)
	assert.Equal(t, cairo1Class.Abi, class.Abi.(string))
	assert.Equal(t, cairo1Class.SemanticVersion, class.ContractClassVersion)

	require.Equal(t, len(cairo1Class.EntryPoints.L1Handler), len(class.EntryPoints.L1Handler))
	for idx := range cairo1Class.EntryPoints.L1Handler {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.L1Handler[idx].Index, *class.EntryPoints.L1Handler[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.L1Handler[idx].Selector, class.EntryPoints.L1Handler[idx].Selector)
	}

	require.Equal(t, len(cairo1Class.EntryPoints.Constructor), len(class.EntryPoints.Constructor))
	for idx := range cairo1Class.EntryPoints.Constructor {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.Constructor[idx].Index, *class.EntryPoints.Constructor[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.Constructor[idx].Selector, class.EntryPoints.Constructor[idx].Selector)
	}

	require.Equal(t, len(cairo1Class.EntryPoints.External), len(class.EntryPoints.External))
	for idx := range cairo1Class.EntryPoints.External {
		assert.Nil(t, class.EntryPoints.External[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.External[idx].Index, *class.EntryPoints.External[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.External[idx].Selector, class.EntryPoints.External[idx].Selector)
	}
}

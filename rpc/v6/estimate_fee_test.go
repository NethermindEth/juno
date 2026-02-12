package rpcv6_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/l1/types"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEstimateMessageFee(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)

	handler := rpc.New(mockReader, nil, mockVM, n, utils.NewNopZapLogger())
	from, err := types.FromString[types.L1Address]("0xDEADBEEF")
	require.NoError(t, err)
	msg := rpc.MsgFromL1{
		From:     from,
		To:       felt.FromUint64[felt.Address](1337),
		Payload:  []felt.Felt{felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](2)},
		Selector: felt.FromUint64[felt.Felt](44),
	}

	t.Run("block not found", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)
		_, err := handler.EstimateMessageFee(msg, rpc.BlockID{Latest: true})
		require.Equal(t, rpccore.ErrBlockNotFound, err)
	})

	latestHeader := &core.Header{
		Number:        9,
		Timestamp:     456,
		L1GasPriceETH: new(felt.Felt).SetUint64(42),
	}
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
	mockReader.EXPECT().HeadsHeader().Return(latestHeader, nil)

	expectedGasConsumed := new(felt.Felt).SetUint64(37)
	mockVM.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), &vm.BlockInfo{
		Header: latestHeader,
	}, gomock.Any(), gomock.Any(), false, true, false, true, true, false).DoAndReturn(
		func(
			txns []core.Transaction,
			declaredClasses []core.ClassDefinition,
			paidFeesOnL1 []*felt.Felt,
			blockInfo *vm.BlockInfo,
			state core.StateReader,
			skipChargeFee,
			skipValidate,
			errOnRevert,
			errStack,
			allowBinarySearch bool,
			isEstimateFee bool,
			returnInitialReads bool,
		) (vm.ExecutionResults, error) {
			require.Len(t, txns, 1)
			assert.NotNil(t, txns[0].(*core.L1HandlerTransaction))

			assert.Empty(t, declaredClasses)
			assert.Len(t, paidFeesOnL1, 1)

			actualFee := new(felt.Felt).Mul(expectedGasConsumed, blockInfo.Header.L1GasPriceETH)
			return vm.ExecutionResults{
				OverallFees:      []*felt.Felt{actualFee},
				DataAvailability: []core.DataAvailability{{L1DataGas: 0}},
				GasConsumed:      []core.GasConsumed{{L1Gas: 37}},
				Traces: []vm.TransactionTrace{{
					StateDiff: &vm.StateDiff{
						StorageDiffs:              []vm.StorageDiff{},
						Nonces:                    []vm.Nonce{},
						DeployedContracts:         []vm.DeployedContract{},
						DeprecatedDeclaredClasses: []*felt.Felt{},
						DeclaredClasses:           []vm.DeclaredClass{},
						ReplacedClasses:           []vm.ReplacedClass{},
					},
				}},
				NumSteps: 0,
			}, nil
		},
	)

	estimateFee, rpcErr := handler.EstimateMessageFee(msg, rpc.BlockID{Latest: true})
	require.Nil(t, rpcErr)
	feeUnit := rpc.WEI
	require.Equal(t, expectedGasConsumed, estimateFee.GasConsumed)
	require.Equal(t, latestHeader.L1GasPriceETH, estimateFee.GasPrice)
	require.Equal(t, new(felt.Felt).
		Mul(expectedGasConsumed, latestHeader.L1GasPriceETH), estimateFee.OverallFee)
	require.Equal(t, feeUnit, *estimateFee.Unit)
}

func assertEqualDeprecatedCairoClass(
	t *testing.T,
	deprecatedCairoClass *core.DeprecatedCairoClass,
	class *rpc.Class,
) {
	assert.Equal(t, deprecatedCairoClass.Program, class.Program)
	assert.Equal(t, deprecatedCairoClass.Abi, class.Abi.(json.RawMessage))

	require.Equal(t, len(deprecatedCairoClass.L1Handlers), len(class.EntryPoints.L1Handler))
	for idx := range deprecatedCairoClass.L1Handlers {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Index)
		assert.Equal(
			t,
			deprecatedCairoClass.L1Handlers[idx].Offset,
			class.EntryPoints.L1Handler[idx].Offset,
		)
		assert.Equal(
			t,
			deprecatedCairoClass.L1Handlers[idx].Selector,
			class.EntryPoints.L1Handler[idx].Selector,
		)
	}

	require.Equal(t, len(deprecatedCairoClass.Constructors), len(class.EntryPoints.Constructor))
	for idx := range deprecatedCairoClass.Constructors {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Index)
		assert.Equal(
			t,
			deprecatedCairoClass.Constructors[idx].Offset,
			class.EntryPoints.Constructor[idx].Offset,
		)
		assert.Equal(
			t,
			deprecatedCairoClass.Constructors[idx].Selector,
			class.EntryPoints.Constructor[idx].Selector,
		)
	}

	require.Equal(t, len(deprecatedCairoClass.Externals), len(class.EntryPoints.External))
	for idx := range deprecatedCairoClass.Externals {
		assert.Nil(t, class.EntryPoints.External[idx].Index)
		assert.Equal(
			t,
			deprecatedCairoClass.Externals[idx].Offset,
			class.EntryPoints.External[idx].Offset,
		)
		assert.Equal(t,
			deprecatedCairoClass.Externals[idx].Selector,
			class.EntryPoints.External[idx].Selector,
		)
	}
}

func assertEqualSierraClass(t *testing.T, sierraClass *core.SierraClass, class *rpc.Class) {
	assert.Equal(t, sierraClass.Program, class.SierraProgram)
	assert.Equal(t, sierraClass.Abi, class.Abi.(string))
	assert.Equal(t, sierraClass.SemanticVersion, class.ContractClassVersion)

	require.Equal(t, len(sierraClass.EntryPoints.L1Handler), len(class.EntryPoints.L1Handler))
	for idx := range sierraClass.EntryPoints.L1Handler {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Offset)
		assert.Equal(
			t,
			sierraClass.EntryPoints.L1Handler[idx].Index,
			*class.EntryPoints.L1Handler[idx].Index,
		)
		assert.Equal(
			t,
			sierraClass.EntryPoints.L1Handler[idx].Selector,
			class.EntryPoints.L1Handler[idx].Selector,
		)
	}

	require.Equal(t, len(sierraClass.EntryPoints.Constructor), len(class.EntryPoints.Constructor))
	for idx := range sierraClass.EntryPoints.Constructor {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Offset)
		assert.Equal(
			t,
			sierraClass.EntryPoints.Constructor[idx].Index,
			*class.EntryPoints.Constructor[idx].Index,
		)
		assert.Equal(
			t,
			sierraClass.EntryPoints.Constructor[idx].Selector,
			class.EntryPoints.Constructor[idx].Selector,
		)
	}

	require.Equal(t, len(sierraClass.EntryPoints.External), len(class.EntryPoints.External))
	for idx := range sierraClass.EntryPoints.External {
		assert.Nil(t, class.EntryPoints.External[idx].Offset)
		assert.Equal(
			t,
			sierraClass.EntryPoints.External[idx].Index,
			*class.EntryPoints.External[idx].Index,
		)
		assert.Equal(
			t,
			sierraClass.EntryPoints.External[idx].Selector,
			class.EntryPoints.External[idx].Selector,
		)
	}
}

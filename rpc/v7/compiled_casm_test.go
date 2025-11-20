package rpcv7_test

import (
	"encoding/json"
	"fmt"
	"testing"

	clientFeeder "github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v7"
	"github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCompiledCasm(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	rd := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(rd, nil, nil, nil, nil)

	t.Run("db failure", func(t *testing.T) {
		rd.EXPECT().HeadState().Return(nil, nil, fmt.Errorf("error"))
		resp, err := handler.CompiledCasm(felt.NewUnsafeFromString[felt.Felt]("0x000"))
		assert.Nil(t, resp)
		assert.Equal(t, jsonrpc.InternalError, err.Code)
	})
	t.Run("class doesn't exist", func(t *testing.T) {
		classHash := felt.NewUnsafeFromString[felt.Felt]("0x111")

		mockState := mocks.NewMockCommonState(mockCtrl)
		mockState.EXPECT().Class(classHash).Return(nil, db.ErrKeyNotFound)
		rd.EXPECT().HeadState().Return(mockState, nopCloser, nil)

		resp, err := handler.CompiledCasm(classHash)
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrClassHashNotFound, err)
	})
	t.Run("deprecatedCairo", func(t *testing.T) {
		classHash := felt.NewUnsafeFromString[felt.Felt]("0x5f18f9cdc05da87f04e8e7685bd346fc029f977167d5b1b2b59f69a7dacbfc8")

		cl := clientFeeder.NewTestClient(t, &utils.Sepolia)
		fd := feeder.New(cl)

		class, err := fd.Class(t.Context(), classHash)
		require.NoError(t, err)

		deprecatedCairo, ok := class.(*core.DeprecatedCairoClass)
		require.True(t, ok)
		program, err := utils.Gzip64Decode(deprecatedCairo.Program)
		require.NoError(t, err)

		// only fields that need to be unmarshaled specified
		var deprecatedCairoDefinition struct {
			Data []*felt.Felt `json:"data"`
		}
		err = json.Unmarshal(program, &deprecatedCairoDefinition)
		require.NoError(t, err)

		mockState := mocks.NewMockCommonState(mockCtrl)
		mockState.EXPECT().Class(classHash).Return(&core.DeclaredClassDefinition{Class: class}, nil)
		rd.EXPECT().HeadState().Return(mockState, nopCloser, nil)

		resp, rpcErr := handler.CompiledCasm(classHash)
		require.Nil(t, rpcErr)
		assert.Equal(t, &rpc.CasmCompiledContractClass{
			Prime:           "0x800000000000011000000000000000000000000000000000000000000000001",
			CompilerVersion: "0.10.3",
			EntryPointsByType: rpc.EntryPointsByType{
				Constructor: utils.Map(deprecatedCairo.Constructors, adaptEntryPoint),
				External:    utils.Map(deprecatedCairo.Externals, adaptEntryPoint),
				L1Handler:   utils.Map(deprecatedCairo.L1Handlers, adaptEntryPoint),
			},
			Hints:    json.RawMessage(`[[2,[{"Dst":0}]]]`),
			Bytecode: deprecatedCairoDefinition.Data,
		}, resp)
	})
}

func adaptEntryPoint(point core.DeprecatedEntryPoint) rpc.CasmEntryPoint {
	return rpc.CasmEntryPoint{
		Offset:   point.Offset,
		Selector: point.Selector,
		Builtins: nil,
	}
}

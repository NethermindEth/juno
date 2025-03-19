package rpcv8_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	clientFeeder "github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v8"
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
	handler := rpc.New(rd, nil, nil, "", nil)

	t.Run("db failure", func(t *testing.T) {
		rd.EXPECT().HeadState().Return(nil, nil, fmt.Errorf("error"))
		resp, err := handler.CompiledCasm(utils.HexToFelt(t, "0x000"))
		assert.Nil(t, resp)
		assert.Equal(t, jsonrpc.InternalError, err.Code)
	})
	t.Run("class doesn't exist", func(t *testing.T) {
		classHash := utils.HexToFelt(t, "0x111")

		mockState := mocks.NewMockStateHistoryReader(mockCtrl)
		mockState.EXPECT().Class(classHash).Return(nil, db.ErrKeyNotFound)
		rd.EXPECT().HeadState().Return(mockState, nopCloser, nil)

		resp, err := handler.CompiledCasm(classHash)
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrClassHashNotFound, err)
	})
	t.Run("cairo0", func(t *testing.T) {
		classHash := utils.HexToFelt(t, "0x5f18f9cdc05da87f04e8e7685bd346fc029f977167d5b1b2b59f69a7dacbfc8")

		cl := clientFeeder.NewTestClient(t, &utils.Sepolia)
		fd := feeder.New(cl)

		class, err := fd.Class(t.Context(), classHash)
		require.NoError(t, err)

		cairo0, ok := class.(*core.Cairo0Class)
		require.True(t, ok)
		program, err := utils.Gzip64Decode(cairo0.Program)
		require.NoError(t, err)

		// only fields that need to be unmarshaled specified
		var cairo0Definition struct {
			Data []*felt.Felt `json:"data"`
		}
		err = json.Unmarshal(program, &cairo0Definition)
		require.NoError(t, err)

		mockState := mocks.NewMockStateHistoryReader(mockCtrl)
		mockState.EXPECT().Class(classHash).Return(&core.DeclaredClass{Class: class}, nil)
		rd.EXPECT().HeadState().Return(mockState, nopCloser, nil)

		resp, rpcErr := handler.CompiledCasm(classHash)
		require.Nil(t, rpcErr)
		assert.Equal(t, &rpc.CasmCompiledContractClass{
			Prime:           "0x800000000000011000000000000000000000000000000000000000000000001",
			CompilerVersion: "0.10.3",
			EntryPointsByType: rpc.EntryPointsByType{
				Constructor: utils.Map(cairo0.Constructors, adaptEntryPoint),
				External:    utils.Map(cairo0.Externals, adaptEntryPoint),
				L1Handler:   utils.Map(cairo0.L1Handlers, adaptEntryPoint),
			},
			Hints:    json.RawMessage(`[[2,[{"Dst":0}]]]`),
			Bytecode: cairo0Definition.Data,
		}, resp)
	})

	t.Run("cairo1", func(t *testing.T) {
		classHash := utils.HexToFelt(t, "0x222")

		// Create a compiled class with test data
		compiledClass := &core.CompiledClass{
			CompilerVersion: "1.0.0",
			Prime:           big.NewInt(123),
			External: []core.CompiledEntryPoint{
				{
					Offset:   42, // Test the uint64 offset
					Selector: utils.HexToFelt(t, "0xabc"),
					Builtins: []string{"range_check"},
				},
			},
			Constructor: []core.CompiledEntryPoint{},
			L1Handler:   []core.CompiledEntryPoint{},
			Bytecode:    []*felt.Felt{utils.HexToFelt(t, "0x123")},
		}

		cairo1Class := &core.Cairo1Class{
			Compiled: compiledClass,
		}

		mockState := mocks.NewMockStateHistoryReader(mockCtrl)
		mockState.EXPECT().Class(classHash).Return(&core.DeclaredClass{Class: cairo1Class}, nil)
		rd.EXPECT().HeadState().Return(mockState, nopCloser, nil)

		resp, rpcErr := handler.CompiledCasm(classHash)
		require.Nil(t, rpcErr)

		// Verify that the offset is correctly passed as uint64
		require.Len(t, resp.EntryPointsByType.External, 1)
		assert.Equal(t, uint64(42), resp.EntryPointsByType.External[0].Offset)
		assert.Equal(t, utils.HexToFelt(t, "0xabc"), resp.EntryPointsByType.External[0].Selector)
		assert.Equal(t, []string{"range_check"}, resp.EntryPointsByType.External[0].Builtins)
		assert.Equal(t, utils.ToHex(big.NewInt(123)), resp.Prime)
		assert.Equal(t, "1.0.0", resp.CompilerVersion)
	})
}

func adaptEntryPoint(point core.EntryPoint) rpc.CasmEntryPoint {
	return rpc.CasmEntryPoint{
		Offset:   point.Offset.Uint64(),
		Selector: point.Selector,
		Builtins: nil,
	}
}

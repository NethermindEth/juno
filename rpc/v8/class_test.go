package rpcv8_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestClass(t *testing.T) {
	n := &utils.Integration
	integrationClient := feeder.NewTestClient(t, n)
	integGw := adaptfeeder.New(integrationClient)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockState := mocks.NewMockStateReader(mockCtrl)

	mockState.EXPECT().Class(
		gomock.Any()).DoAndReturn(func(
		classHash *felt.Felt,
	) (*core.DeclaredClassDefinition, error) {
		class, err := integGw.Class(t.Context(), classHash)
		return &core.DeclaredClassDefinition{Class: class, At: 0}, err
	}).AnyTimes()
	mockReader.EXPECT().HeadState().Return(mockState, func() error {
		return nil
	}, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil).AnyTimes()
	handler := rpc.New(mockReader, nil, nil, utils.NewNopZapLogger())

	latest := blockIDLatest(t)

	t.Run("sierra class", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.Felt](
			"0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5",
		)

		coreClass, err := integGw.Class(t.Context(), hash)
		require.NoError(t, err)

		class, rpcErr := handler.Class(latest, *hash)
		require.Nil(t, rpcErr)
		sierraClass := coreClass.(*core.SierraClass)
		assertEqualSierraClass(t, sierraClass, class)
	})

	t.Run("casm class", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.Felt](
			"0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04",
		)

		coreClass, err := integGw.Class(t.Context(), hash)
		require.NoError(t, err)

		class, rpcErr := handler.Class(latest, *hash)
		require.Nil(t, rpcErr)

		deprecatedCairoClass := coreClass.(*core.DeprecatedCairoClass)
		assertEqualDeprecatedCairoClass(t, deprecatedCairoClass, class)
	})

	t.Run("state by id error", func(t *testing.T) {
		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, utils.NewNopZapLogger())

		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		_, rpcErr := handler.Class(latest, felt.Zero)
		require.NotNil(t, rpcErr)
		require.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("class hash not found error", func(t *testing.T) {
		mockReader := mocks.NewMockReader(mockCtrl)
		mockState := mocks.NewMockStateReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, utils.NewNopZapLogger())

		mockReader.EXPECT().HeadState().Return(mockState, func() error {
			return nil
		}, nil)
		mockState.EXPECT().Class(gomock.Any()).Return(nil, errors.New("class hash not found"))

		_, rpcErr := handler.Class(latest, felt.Zero)
		require.NotNil(t, rpcErr)
		require.Equal(t, rpccore.ErrClassHashNotFound, rpcErr)
	})
}

func TestClassAt(t *testing.T) {
	n := &utils.Integration
	integrationClient := feeder.NewTestClient(t, n)
	integGw := adaptfeeder.New(integrationClient)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockState := mocks.NewMockStateReader(mockCtrl)

	deprecatedCairoContractAddress := felt.NewRandom[felt.Felt]()
	deprecatedCairoClassHash := felt.NewUnsafeFromString[felt.Felt](
		"0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04",
	)
	mockState.EXPECT().
		ContractClassHash(deprecatedCairoContractAddress).
		Return(*deprecatedCairoClassHash, nil)

	cairo1ContractAddress := felt.NewRandom[felt.Felt]()
	sierraClassHash := felt.NewUnsafeFromString[felt.Felt](
		"0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5",
	)
	mockState.EXPECT().ContractClassHash(cairo1ContractAddress).Return(*sierraClassHash, nil)

	mockState.EXPECT().Class(gomock.Any()).
		DoAndReturn(func(classHash *felt.Felt) (*core.DeclaredClassDefinition, error) {
			class, err := integGw.Class(t.Context(), classHash)
			return &core.DeclaredClassDefinition{Class: class, At: 0}, err
		}).AnyTimes()
	mockReader.EXPECT().HeadState().Return(mockState, func() error {
		return nil
	}, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil).AnyTimes()
	handler := rpc.New(mockReader, nil, nil, utils.NewNopZapLogger())

	latest := blockIDLatest(t)

	t.Run("sierra class", func(t *testing.T) {
		coreClass, err := integGw.Class(t.Context(), sierraClassHash)
		require.NoError(t, err)

		class, rpcErr := handler.ClassAt(latest, *cairo1ContractAddress)
		require.Nil(t, rpcErr)
		sierraClass := coreClass.(*core.SierraClass)
		assertEqualSierraClass(t, sierraClass, class)
	})

	t.Run("casm class", func(t *testing.T) {
		coreClass, err := integGw.Class(t.Context(), deprecatedCairoClassHash)
		require.NoError(t, err)

		class, rpcErr := handler.ClassAt(latest, *deprecatedCairoContractAddress)
		require.Nil(t, rpcErr)

		deprecatedCairoClass := coreClass.(*core.DeprecatedCairoClass)
		assertEqualDeprecatedCairoClass(t, deprecatedCairoClass, class)
	})
}

func TestClassHashAt(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, log)

	latestID := blockIDLatest(t)
	targetAddress := felt.FromUint64[felt.Felt](1234)
	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		classHash, rpcErr := handler.ClassHashAt(latestID, targetAddress)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		classHash, rpcErr := handler.ClassHashAt(
			blockIDHash(t, &felt.Zero),
			targetAddress,
		)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		classHash, rpcErr := handler.ClassHashAt(
			blockIDNumber(t, 0),
			targetAddress,
		)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).
			Return(felt.Felt{}, errors.New("non-existent contract"))

		classHash, rpcErr := handler.ClassHashAt(latestID, targetAddress)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	expectedClassHash := new(felt.Felt).SetUint64(3)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).
			Return(*expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(latestID, targetAddress)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(*expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(
			blockIDHash(t, &felt.Zero),
			targetAddress,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(*expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(
			blockIDNumber(t, 0),
			targetAddress,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(*expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(
			blockIDPending(t),
			targetAddress,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})
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

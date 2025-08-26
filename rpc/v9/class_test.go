package rpcv9_test

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
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
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
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	mockState.EXPECT().
		Class(gomock.Any()).
		DoAndReturn(func(classHash *felt.Felt) (*core.DeclaredClass, error) {
			class, err := integGw.Class(t.Context(), classHash)
			return &core.DeclaredClass{Class: class, At: 0}, err
		}).
		AnyTimes()
	mockReader.EXPECT().HeadState().Return(mockState, func() error {
		return nil
	}, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil).AnyTimes()
	handler := rpcv9.New(mockReader, nil, nil, utils.NewNopZapLogger())

	latest := blockIDLatest(t)

	t.Run("sierra class", func(t *testing.T) {
		hash := utils.HexToFelt(
			t,
			"0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5",
		)

		coreClass, err := integGw.Class(t.Context(), hash)
		require.NoError(t, err)

		class, rpcErr := handler.Class(&latest, hash)
		require.Nil(t, rpcErr)
		cairo1Class := coreClass.(*core.Cairo1Class)
		assertEqualCairo1Class(t, cairo1Class, class)
	})

	t.Run("casm class", func(t *testing.T) {
		hash := utils.HexToFelt(
			t,
			"0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04",
		)

		coreClass, err := integGw.Class(t.Context(), hash)
		require.NoError(t, err)

		class, rpcErr := handler.Class(&latest, hash)
		require.Nil(t, rpcErr)

		cairo0Class := coreClass.(*core.Cairo0Class)
		assertEqualCairo0Class(t, cairo0Class, class)
	})

	t.Run("state by id error", func(t *testing.T) {
		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpcv9.New(mockReader, nil, nil, utils.NewNopZapLogger())

		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		_, rpcErr := handler.Class(&latest, &felt.Zero)
		require.NotNil(t, rpcErr)
		require.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("class hash not found error", func(t *testing.T) {
		mockReader := mocks.NewMockReader(mockCtrl)
		mockState := mocks.NewMockStateHistoryReader(mockCtrl)
		handler := rpcv9.New(mockReader, nil, nil, utils.NewNopZapLogger())

		mockReader.EXPECT().HeadState().Return(mockState, func() error {
			return nil
		}, nil)
		mockState.EXPECT().Class(gomock.Any()).Return(nil, errors.New("class hash not found"))

		_, rpcErr := handler.Class(&latest, &felt.Zero)
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
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	cairo0ContractAddress, _ := new(felt.Felt).SetRandom()
	cairo0ClassHash := utils.HexToFelt(
		t,
		"0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04",
	)
	mockState.EXPECT().ContractClassHash(cairo0ContractAddress).Return(cairo0ClassHash, nil)

	cairo1ContractAddress, _ := new(felt.Felt).SetRandom()
	cairo1ClassHash := utils.HexToFelt(
		t,
		"0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5",
	)
	mockState.EXPECT().ContractClassHash(cairo1ContractAddress).Return(cairo1ClassHash, nil)

	mockState.EXPECT().
		Class(gomock.Any()).
		DoAndReturn(func(classHash *felt.Felt) (*core.DeclaredClass, error) {
			class, err := integGw.Class(t.Context(), classHash)
			return &core.DeclaredClass{Class: class, At: 0}, err
		}).
		AnyTimes()
	mockReader.EXPECT().HeadState().Return(mockState, func() error {
		return nil
	}, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil).AnyTimes()
	handler := rpcv9.New(mockReader, nil, nil, utils.NewNopZapLogger())

	latest := blockIDLatest(t)

	t.Run("sierra class", func(t *testing.T) {
		coreClass, err := integGw.Class(t.Context(), cairo1ClassHash)
		require.NoError(t, err)

		class, rpcErr := handler.ClassAt(&latest, cairo1ContractAddress)
		require.Nil(t, rpcErr)
		cairo1Class := coreClass.(*core.Cairo1Class)
		assertEqualCairo1Class(t, cairo1Class, class)
	})

	t.Run("casm class", func(t *testing.T) {
		coreClass, err := integGw.Class(t.Context(), cairo0ClassHash)
		require.NoError(t, err)

		class, rpcErr := handler.ClassAt(&latest, cairo0ContractAddress)
		require.Nil(t, rpcErr)

		cairo0Class := coreClass.(*core.Cairo0Class)
		assertEqualCairo0Class(t, cairo0Class, class)
	})
}

func TestClassHashAt(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpcv9.New(mockReader, mockSyncReader, nil, log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)
		latest := blockIDLatest(t)
		classHash, rpcErr := handler.ClassHashAt(&latest, &felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)
		hash := blockIDHash(t, &felt.Zero)
		classHash, rpcErr := handler.ClassHashAt(&hash, &felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)
		number := blockIDNumber(t, 0)
		classHash, rpcErr := handler.ClassHashAt(&number, &felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().
			ContractClassHash(gomock.Any()).
			Return(nil, errors.New("non-existent contract"))
		latest := blockIDLatest(t)
		classHash, rpcErr := handler.ClassHashAt(&latest, &felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	expectedClassHash := new(felt.Felt).SetUint64(3)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(expectedClassHash, nil)
		latest := blockIDLatest(t)
		classHash, rpcErr := handler.ClassHashAt(&latest, &felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(expectedClassHash, nil)
		hash := blockIDHash(t, &felt.Zero)
		classHash, rpcErr := handler.ClassHashAt(&hash, &felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(expectedClassHash, nil)
		number := blockIDNumber(t, 0)
		classHash, rpcErr := handler.ClassHashAt(&number, &felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - pre_confirmed", func(t *testing.T) {
		mockSyncReader.EXPECT().PendingState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(expectedClassHash, nil)
		preConfirmed := blockIDPreConfirmed(t)
		classHash, rpcErr := handler.ClassHashAt(&preConfirmed, &felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - l1_accepted", func(t *testing.T) {
		l1HeadBlockNumber := uint64(10)
		mockReader.EXPECT().L1Head().Return(
			&core.L1Head{
				BlockNumber: l1HeadBlockNumber,
				BlockHash:   &felt.Zero,
				StateRoot:   &felt.Zero,
			},
			nil,
		)
		mockReader.EXPECT().StateAtBlockNumber(l1HeadBlockNumber).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(expectedClassHash, nil)
		l1AcceptedID := blockIDL1Accepted(t)
		classHash, rpcErr := handler.ClassHashAt(&l1AcceptedID, &felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})
}

func assertEqualCairo0Class(t *testing.T, cairo0Class *core.Cairo0Class, class *rpcv6.Class) {
	assert.Equal(t, cairo0Class.Program, class.Program)
	assert.Equal(t, cairo0Class.Abi, class.Abi.(json.RawMessage))

	require.Equal(t, len(cairo0Class.L1Handlers), len(class.EntryPoints.L1Handler))
	for idx := range cairo0Class.L1Handlers {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Index)
		assert.Equal(t, cairo0Class.L1Handlers[idx].Offset, class.EntryPoints.L1Handler[idx].Offset)
		assert.Equal(
			t,
			cairo0Class.L1Handlers[idx].Selector,
			class.EntryPoints.L1Handler[idx].Selector,
		)
	}

	require.Equal(t, len(cairo0Class.Constructors), len(class.EntryPoints.Constructor))
	for idx := range cairo0Class.Constructors {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Index)
		assert.Equal(
			t,
			cairo0Class.Constructors[idx].Offset,
			class.EntryPoints.Constructor[idx].Offset,
		)
		assert.Equal(
			t,
			cairo0Class.Constructors[idx].Selector,
			class.EntryPoints.Constructor[idx].Selector,
		)
	}

	require.Equal(t, len(cairo0Class.Externals), len(class.EntryPoints.External))
	for idx := range cairo0Class.Externals {
		assert.Nil(t, class.EntryPoints.External[idx].Index)
		assert.Equal(t, cairo0Class.Externals[idx].Offset, class.EntryPoints.External[idx].Offset)
		assert.Equal(
			t,
			cairo0Class.Externals[idx].Selector,
			class.EntryPoints.External[idx].Selector,
		)
	}
}

func assertEqualCairo1Class(t *testing.T, cairo1Class *core.Cairo1Class, class *rpcv6.Class) {
	assert.Equal(t, cairo1Class.Program, class.SierraProgram)
	assert.Equal(t, cairo1Class.Abi, class.Abi.(string))
	assert.Equal(t, cairo1Class.SemanticVersion, class.ContractClassVersion)

	require.Equal(t, len(cairo1Class.EntryPoints.L1Handler), len(class.EntryPoints.L1Handler))
	for idx := range cairo1Class.EntryPoints.L1Handler {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Offset)
		assert.Equal(
			t,
			cairo1Class.EntryPoints.L1Handler[idx].Index,
			*class.EntryPoints.L1Handler[idx].Index,
		)
		assert.Equal(
			t,
			cairo1Class.EntryPoints.L1Handler[idx].Selector,
			class.EntryPoints.L1Handler[idx].Selector,
		)
	}

	require.Equal(t, len(cairo1Class.EntryPoints.Constructor), len(class.EntryPoints.Constructor))
	for idx := range cairo1Class.EntryPoints.Constructor {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Offset)
		assert.Equal(
			t,
			cairo1Class.EntryPoints.Constructor[idx].Index,
			*class.EntryPoints.Constructor[idx].Index,
		)
		assert.Equal(
			t,
			cairo1Class.EntryPoints.Constructor[idx].Selector,
			class.EntryPoints.Constructor[idx].Selector,
		)
	}

	require.Equal(t, len(cairo1Class.EntryPoints.External), len(class.EntryPoints.External))
	for idx := range cairo1Class.EntryPoints.External {
		assert.Nil(t, class.EntryPoints.External[idx].Offset)
		assert.Equal(
			t,
			cairo1Class.EntryPoints.External[idx].Index,
			*class.EntryPoints.External[idx].Index,
		)
		assert.Equal(
			t,
			cairo1Class.EntryPoints.External[idx].Selector,
			class.EntryPoints.External[idx].Selector,
		)
	}
}

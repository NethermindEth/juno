package rpcv6_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v6"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
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

	mockState.EXPECT().Class(gomock.Any()).DoAndReturn(func(classHash *felt.Felt) (*core.DeclaredClass, error) {
		class, err := integGw.Class(t.Context(), classHash)
		return &core.DeclaredClass{Class: class, At: 0}, err
	}).AnyTimes()
	mockReader.EXPECT().HeadState().Return(mockState, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil).AnyTimes()
	handler := rpc.New(mockReader, nil, nil, n, utils.NewNopZapLogger())

	latest := rpc.BlockID{Latest: true}

	t.Run("sierra class", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5")

		coreClass, err := integGw.Class(t.Context(), hash)
		require.NoError(t, err)

		class, rpcErr := handler.Class(latest, *hash)
		require.Nil(t, rpcErr)
		cairo1Class := coreClass.(*core.Cairo1Class)
		assertEqualCairo1Class(t, cairo1Class, class)
	})

	t.Run("casm class", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04")

		coreClass, err := integGw.Class(t.Context(), hash)
		require.NoError(t, err)

		class, rpcErr := handler.Class(latest, *hash)
		require.Nil(t, rpcErr)

		cairo0Class := coreClass.(*core.Cairo0Class)
		assertEqualCairo0Class(t, cairo0Class, class)
	})

	t.Run("state by id error", func(t *testing.T) {
		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, n, utils.NewNopZapLogger())

		mockReader.EXPECT().HeadState().Return(nil, db.ErrKeyNotFound)

		_, rpcErr := handler.Class(latest, felt.Zero)
		require.NotNil(t, rpcErr)
		require.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("class hash not found error", func(t *testing.T) {
		mockReader := mocks.NewMockReader(mockCtrl)
		mockState := mocks.NewMockStateReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, "", n, utils.NewNopZapLogger())

		mockReader.EXPECT().HeadState().Return(mockState, nil)
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

	cairo0ContractAddress, _ := new(felt.Felt).SetRandom()
	cairo0ClassHash := utils.HexToFelt(t, "0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04")
	mockState.EXPECT().ContractClassHash(cairo0ContractAddress).Return(*cairo0ClassHash, nil)

	cairo1ContractAddress, _ := new(felt.Felt).SetRandom()
	cairo1ClassHash := utils.HexToFelt(t, "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5")
	mockState.EXPECT().ContractClassHash(cairo1ContractAddress).Return(*cairo1ClassHash, nil)

	mockState.EXPECT().Class(gomock.Any()).DoAndReturn(func(classHash *felt.Felt) (*core.DeclaredClass, error) {
		class, err := integGw.Class(t.Context(), classHash)
		return &core.DeclaredClass{Class: class, At: 0}, err
	}).AnyTimes()
	mockReader.EXPECT().HeadState().Return(mockState, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil).AnyTimes()
	handler := rpc.New(mockReader, nil, nil, n, utils.NewNopZapLogger())

	latest := rpc.BlockID{Latest: true}

	t.Run("sierra class", func(t *testing.T) {
		coreClass, err := integGw.Class(t.Context(), cairo1ClassHash)
		require.NoError(t, err)

		class, rpcErr := handler.ClassAt(latest, *cairo1ContractAddress)
		require.Nil(t, rpcErr)
		cairo1Class := coreClass.(*core.Cairo1Class)
		assertEqualCairo1Class(t, cairo1Class, class)
	})

	t.Run("casm class", func(t *testing.T) {
		coreClass, err := integGw.Class(t.Context(), cairo0ClassHash)
		require.NoError(t, err)

		class, rpcErr := handler.ClassAt(latest, *cairo0ContractAddress)
		require.Nil(t, rpcErr)

		cairo0Class := coreClass.(*core.Cairo0Class)
		assertEqualCairo0Class(t, cairo0Class, class)
	})
}

func TestClassHashAt(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, n, log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, db.ErrKeyNotFound)

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, db.ErrKeyNotFound)

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Hash: &felt.Zero}, felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, db.ErrKeyNotFound)

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Number: 0}, felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(felt.Zero, errors.New("non-existent contract"))

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	expectedClassHash := new(felt.Felt).SetUint64(3)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(*expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(*expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Hash: &felt.Zero}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(*expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Number: 0}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		pending := sync.NewPending(nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(&pending, nil)
		mockSyncReader.EXPECT().PendingState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Pending: true}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})
}

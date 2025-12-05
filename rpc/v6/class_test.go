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
	handler := rpc.New(mockReader, nil, nil, n, utils.NewNopZapLogger())

	latest := rpc.BlockID{Latest: true}

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
		handler := rpc.New(mockReader, nil, nil, n, utils.NewNopZapLogger())

		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		_, rpcErr := handler.Class(latest, felt.Zero)
		require.NotNil(t, rpcErr)
		require.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("class hash not found error", func(t *testing.T) {
		mockReader := mocks.NewMockReader(mockCtrl)
		mockState := mocks.NewMockStateReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, n, utils.NewNopZapLogger())

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
	handler := rpc.New(mockReader, nil, nil, n, utils.NewNopZapLogger())

	latest := rpc.BlockID{Latest: true}

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

	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, n, log)

	latestID := rpc.BlockID{Latest: true}
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
			rpc.BlockID{Hash: &felt.Zero},
			targetAddress,
		)
		require.Nil(t, classHash)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		classHash, rpcErr := handler.ClassHashAt(
			rpc.BlockID{Number: 0},
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
			rpc.BlockID{Hash: &felt.Zero},
			targetAddress,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(*expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(
			rpc.BlockID{Number: 0},
			targetAddress,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	//nolint:dupl //  similar structure with nonce test, different endpoint.
	t.Run("blockID - pending", func(t *testing.T) {
		pendingStateDiff := core.EmptyStateDiff()
		pendingStateDiff.DeployedContracts[targetAddress] = expectedClassHash

		pending := core.Pending{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: felt.NewFromUint64[felt.Felt](2),
				},
			},
			StateUpdate: &core.StateUpdate{
				StateDiff: &pendingStateDiff,
			},
		}
		mockSyncReader.EXPECT().PendingData().Return(&pending, nil)
		mockReader.EXPECT().StateAtBlockHash(pending.Block.ParentHash).
			Return(mockState, nopCloser, nil)

		classHash, rpcErr := handler.ClassHashAt(
			rpc.BlockID{Pending: true},
			targetAddress,
		)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})
}

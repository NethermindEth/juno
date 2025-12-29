package plugin_test

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	junoplugin "github.com/NethermindEth/juno/plugin"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPlugin(t *testing.T) {
	timeout := time.Second
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	plugin := mocks.NewMockJunoPlugin(mockCtrl)

	mainClient := feeder.NewTestClient(t, &utils.Mainnet)
	mainGw := adaptfeeder.New(mainClient)

	integClient := feeder.NewTestClient(t, &utils.Integration)
	integGw := adaptfeeder.New(integClient)

	testDB := memory.New()

	// sync to integration for 2 blocks
	for i := range 2 {
		su, block, err := integGw.StateUpdateWithBlock(t.Context(), uint64(i))
		require.NoError(t, err)
		plugin.EXPECT().NewBlock(block, su, gomock.Any())
	}
	bc := blockchain.New(testDB, &utils.Integration, statetestutils.UseNewState())
	dataSource := sync.NewFeederGatewayDataSource(bc, integGw)
	synchronizer := sync.New(bc, dataSource, utils.NewNopZapLogger(), 0, 0, false, nil).WithPlugin(plugin)

	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	require.NoError(t, synchronizer.Run(ctx))
	cancel()
	require.NoError(t, bc.Stop())

	t.Run("resync to mainnet with the same db", func(t *testing.T) {
		bc := blockchain.New(testDB, &utils.Mainnet, statetestutils.UseNewState())

		// Ensure current head is Integration head
		head, err := bc.HeadsHeader()
		require.NoError(t, err)
		require.Equal(t, felt.NewUnsafeFromString[felt.Felt]("0x34e815552e42c5eb5233b99de2d3d7fd396e575df2719bf98e7ed2794494f86"), head.Hash)

		// Reorg 2 blocks, then sync 3 blocks
		su1, block1, err := integGw.StateUpdateWithBlock(t.Context(), uint64(1))
		require.NoError(t, err)
		su0, block0, err := integGw.StateUpdateWithBlock(t.Context(), uint64(0))
		require.NoError(t, err)
		plugin.EXPECT().RevertBlock(&junoplugin.BlockAndStateUpdate{block1, su1}, &junoplugin.BlockAndStateUpdate{block0, su0}, gomock.Any())
		plugin.EXPECT().RevertBlock(&junoplugin.BlockAndStateUpdate{block0, su0}, nil, gomock.Any())
		for i := range 3 {
			su, block, err := mainGw.StateUpdateWithBlock(t.Context(), uint64(i))
			require.NoError(t, err)
			plugin.EXPECT().NewBlock(block, su, gomock.Any())
		}

		dataSource := sync.NewFeederGatewayDataSource(bc, mainGw)
		synchronizer = sync.New(bc, dataSource, utils.NewNopZapLogger(), 0, 0, false, nil).WithPlugin(plugin)
		ctx, cancel = context.WithTimeout(t.Context(), timeout)
		require.NoError(t, synchronizer.Run(ctx))
		cancel()

		// After syncing (and reorging) the current head should be at mainnet
		head, err = bc.HeadsHeader()
		require.NoError(t, err)
		require.Equal(t, felt.NewUnsafeFromString[felt.Felt]("0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6"), head.Hash)
	})
}

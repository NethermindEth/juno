package main_test

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	juno "github.com/NethermindEth/juno/cmd/juno"
	"github.com/NethermindEth/juno/core"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db/pebble"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var emptyCommitments = core.BlockCommitments{}

func TestDBCmd(t *testing.T) {
	t.Run("retrieve info when db contains block0", func(t *testing.T) {
		cmd := juno.DBInfoCmd()
		if statetestutils.UseNewState() {
			require.NoError(t, cmd.Flags().Set("new-state", "true"))
		}
		executeCmdInDB(t, cmd)
	})

	t.Run("inspect db when db contains block0", func(t *testing.T) {
		cmd := juno.DBSizeCmd()
		executeCmdInDB(t, cmd)
	})

	t.Run("revert db by 1 block", func(t *testing.T) {
		network := utils.Mainnet

		const (
			syncToBlock   = uint64(2)
			revertToBlock = syncToBlock - 1
		)

		cmd := juno.DBRevertCmd()
		cmd.Flags().String("db-path", "", "")

		dbPath := prepareDB(t, &network, syncToBlock)

		require.NoError(t, cmd.Flags().Set("db-path", dbPath))
		require.NoError(t, cmd.Flags().Set("to-block", strconv.Itoa(int(revertToBlock))))
		if statetestutils.UseNewState() {
			require.NoError(t, cmd.Flags().Set("new-state", "true"))
		}
		require.NoError(t, cmd.Execute())

		// unfortunately we cannot use blockchain from prepareDB because
		// inside revert cmd another pebble instance is used which will panic if there are other instances
		// that use the same db path
		db, err := pebble.New(dbPath)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, db.Close())
		})

		chain := blockchain.New(db, &network, statetestutils.UseNewState())
		block, err := chain.Head()
		require.NoError(t, err)
		assert.Equal(t, revertToBlock, block.Number)
	})
}

func executeCmdInDB(t *testing.T, cmd *cobra.Command) {
	cmd.Flags().String("db-path", "", "")

	dbPath := prepareDB(t, &utils.Mainnet, 0)

	require.NoError(t, cmd.Flags().Set("db-path", dbPath))
	require.NoError(t, cmd.Execute())
}

func prepareDB(t *testing.T, network *utils.Network, syncToBlock uint64) string {
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)

	dbPath := t.TempDir()
	testDB, err := pebble.New(dbPath)
	require.NoError(t, err)

	chain := blockchain.New(testDB, network, statetestutils.UseNewState())

	for blockNumber := uint64(0); blockNumber <= syncToBlock; blockNumber++ {
		block, err := gw.BlockByNumber(t.Context(), blockNumber)
		require.NoError(t, err)

		stateUpdate, err := gw.StateUpdate(t.Context(), blockNumber)
		require.NoError(t, err)

		require.NoError(t, chain.Store(block, &emptyCommitments, stateUpdate, nil))
	}
	require.NoError(t, chain.Stop())
	require.NoError(t, testDB.Close())

	return dbPath
}

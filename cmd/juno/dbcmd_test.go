package main_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	juno "github.com/NethermindEth/juno/cmd/juno"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db/pebble"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

var emptyCommitments = core.BlockCommitments{}

func TestDBCmd(t *testing.T) {
	t.Run("retrieve info when db contains block0", func(t *testing.T) {
		cmd := juno.DBInfoCmd()
		executeCmdInDB(t, cmd)
	})

	t.Run("inspect db when db contains block0", func(t *testing.T) {
		cmd := juno.DBSizeCmd()
		executeCmdInDB(t, cmd)
	})
}

func executeCmdInDB(t *testing.T, cmd *cobra.Command) {
	cmd.Flags().String("db-path", "", "")

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)
	block0, err := gw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

	stateUpdate0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	dbPath := t.TempDir()
	testDB, err := pebble.New(dbPath)
	require.NoError(t, err)

	chain := blockchain.New(testDB, &utils.Mainnet)
	require.NoError(t, chain.Store(block0, &emptyCommitments, stateUpdate0, nil))
	testDB.Close()

	require.NoError(t, cmd.Flags().Set("db-path", dbPath))
	require.NoError(t, cmd.Execute())
}

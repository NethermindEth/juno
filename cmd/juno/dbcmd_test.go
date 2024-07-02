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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var emptyCommitments = core.BlockCommitments{}

func TestGetDBInfo(t *testing.T) {
	cmd := juno.GetDBInfoCmd()
	cmd.Flags().String("db-path", "", "")
	require.NoError(t, cmd.Execute())

	t.Run("retrieve info when db contains block0", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Mainnet)
		gw := adaptfeeder.New(client)
		block0, err := gw.BlockByNumber(context.Background(), 0)
		require.NoError(t, err)

		stateUpdate0, err := gw.StateUpdate(context.Background(), 0)
		require.NoError(t, err)

		dbPath := t.TempDir()
		testDB, _ := pebble.New(dbPath, 8, 8, nil)
		chain := blockchain.New(testDB, &utils.Mainnet)
		assert.NoError(t, chain.Store(block0, &emptyCommitments, stateUpdate0, nil))
		testDB.Close()

		require.NoError(t, cmd.Flags().Set("db-path", dbPath))
		require.NoError(t, cmd.Execute())
	})
}

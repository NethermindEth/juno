package statebackend

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("new state backend", func(t *testing.T) {
		memDB := memory.New()
		filter := core.NewRunningEventFilterLazy(memDB)
		network := &networks.Mainnet

		backend := New(memDB, filter, network, true)

		sb, ok := backend.(*stateBackend)
		require.True(t, ok, "expected *stateBackend, got %T", backend)
		assert.Equal(t, memDB, sb.baseState.database)
		assert.Equal(t, filter, sb.baseState.runningFilter)
		assert.Equal(t, network, sb.baseState.network)
		assert.NotNil(t, sb.stateDB)
	})

	t.Run("deprecated state backend", func(t *testing.T) {
		memDB := memory.New()
		filter := core.NewRunningEventFilterLazy(memDB)
		network := &networks.Mainnet

		backend := New(memDB, filter, network, false)

		dsb, ok := backend.(*deprecatedStateBackend)
		require.True(t, ok, "expected *deprecatedStateBackend, got %T", backend)
		assert.Equal(t, memDB, dsb.baseState.database)
		assert.Equal(t, filter, dsb.baseState.runningFilter)
		assert.Equal(t, network, dsb.baseState.network)
	})
}

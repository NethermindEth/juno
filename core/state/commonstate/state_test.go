package commonstate

import (
	"context"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateAdapter(t *testing.T) {
	memDB := memory.New()
	db, err := triedb.New(memDB, nil)
	if err != nil {
		panic(err)
	}
	stateDB := state.NewStateDB(memDB, db)
	state, err := state.New(&felt.Zero, stateDB)
	require.NoError(t, err)

	stateAdapter := NewStateAdapter(state)
	assert.NotNil(t, stateAdapter)
}

func TestCoreStateReaderAdapter(t *testing.T) {
	testDB := memory.New()
	txn := testDB.NewIndexedBatch()

	state := core.NewState(txn)
	coreStateReaderAdapter := NewDeprecatedStateReaderAdapter(state)
	assert.NotNil(t, coreStateReaderAdapter)
}

func fetchStateUpdates(samples int) ([]*core.StateUpdate, error) {
	client := feeder.NewClient(utils.Mainnet.FeederURL).
		WithAPIKey("YOUR_API_KEY")
	gw := adaptfeeder.New(client)

	suList := make([]*core.StateUpdate, samples)
	for i := range samples {
		fmt.Println("fetching", i)
		su, err := gw.StateUpdate(context.Background(), uint64(i))
		if err != nil {
			return nil, err
		}
		suList[i] = su
	}
	return suList, nil
}

func BenchmarkStateUpdate(b *testing.B) {
	samples := 50
	suList, err := fetchStateUpdates(samples)
	require.NoError(b, err)

	b.Run("NewState", func(b *testing.B) {
		for b.Loop() {
			b.ReportAllocs()

			b.StopTimer()

			state, stateFactory, txn := prepareState(b, true)

			b.StartTimer()

			for i := range samples {
				declaredClasses := make(map[felt.Felt]core.Class)
				if err := state.Update(uint64(i), suList[i], declaredClasses, false, true); err != nil {
					b.Fatalf("Update failed: %v", err)
				}
				state, err = stateFactory.NewState(suList[i].NewRoot, txn)
				require.NoError(b, err)
			}
		}
	})

	b.Run("OldState", func(b *testing.B) {
		for b.Loop() {
			b.ReportAllocs()

			b.StopTimer()

			state, _, _ := prepareState(b, false)

			b.StartTimer()

			for i := range samples {
				declaredClasses := make(map[felt.Felt]core.Class)
				if err := state.Update(uint64(i), suList[i], declaredClasses, false, true); err != nil {
					b.Fatalf("Update failed: %v", err)
				}
			}
		}
	})
}

func prepareState(b *testing.B, newState bool) (State, *StateFactory, db.IndexedBatch) {
	memDB := memory.New()
	trieDB, err := triedb.New(memDB, nil)
	require.NoError(b, err)
	stateDB := state.NewStateDB(memDB, trieDB)
	txn := memDB.NewIndexedBatch()
	stateFactory, err := NewStateFactory(newState, trieDB, stateDB)
	require.NoError(b, err)

	state, err := stateFactory.NewState(&felt.Zero, txn)
	require.NoError(b, err)

	return state, stateFactory, txn
}

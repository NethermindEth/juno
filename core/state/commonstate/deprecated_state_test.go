package commonstate

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
)

func TestDeprecatedStateAdapter(t *testing.T) {
	testDB := memory.New()
	txn := testDB.NewIndexedBatch()

	state := core.NewState(txn)
	deprecatedStateAdapter := NewDeprecatedStateAdapter(state)
	assert.NotNil(t, deprecatedStateAdapter)
}

func TestDeprecatedStateReaderAdapter(t *testing.T) {
	testDB := memory.New()
	txn := testDB.NewIndexedBatch()

	state := core.NewState(txn)
	deprecatedStateAdapter := NewDeprecatedStateReaderAdapter(state)
	assert.NotNil(t, deprecatedStateAdapter)
}

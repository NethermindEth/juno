package migration_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

// mockMigration is a simple migration implementation for testing
type mockMigration struct {
	beforeCalled      bool
	migrateCalled     bool
	beforeErr         error
	migrateErr        error
	intermediateState []byte
	receivedState     []byte
}

func (m *mockMigration) Before(intermediateState []byte) error {
	m.beforeCalled = true
	m.receivedState = intermediateState
	return m.beforeErr
}

func (m *mockMigration) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	network *utils.Network,
	log utils.StructuredLogger,
) ([]byte, error) {
	m.migrateCalled = true
	return m.intermediateState, m.migrateErr
}

func TestNewRegistry(t *testing.T) {
	r := migration.NewRegistry()
	require.NotNil(t, r)
	require.Equal(t, 0, r.Count())
	require.Equal(t, migration.SchemaVersion(0), r.TargetVersion())
	require.Equal(t, 0, len(r.Entries()))
}

func TestRegistry_With(t *testing.T) {
	r := migration.NewRegistry()
	m := &mockMigration{}

	// Add first mandatory migration
	result := r.With(m)
	require.NotNil(t, result)
	require.Same(t, r, result)
	require.Equal(t, 1, r.Count())
	require.Equal(t, migration.SchemaVersion(1), r.TargetVersion())
	require.Equal(t, m, r.Entries()[0])

	// Add second mandatory migration
	m2 := &mockMigration{}
	result2 := r.With(m2)
	require.NotNil(t, result2)
	require.Same(t, r, result2)
	require.Equal(t, 2, r.Count())
	require.Equal(t, migration.SchemaVersion(3), r.TargetVersion())
	require.Equal(t, m2, r.Entries()[1])
}

func TestRegistry_With_PanicOnMaxMigrations(t *testing.T) {
	r := migration.NewRegistry()
	m := &mockMigration{}

	// Add 64 migrations (indices 0-63)
	for range 64 {
		r.With(m)
	}

	// 65th migration should panic
	require.Panics(
		t,
		func() {
			r.With(m)
		},
		"expected panic when exceeding 64 migrations",
	)
}

func TestRegistry_WithOptional(t *testing.T) {
	m := &mockMigration{}

	t.Run("Enabled false", func(t *testing.T) {
		r := migration.NewRegistry()
		result := r.WithOptional(m, false)
		require.Same(t, r, result)
		require.Equal(t, 1, r.Count())
		require.Equal(t, migration.SchemaVersion(0), r.TargetVersion())
		require.Equal(t, m, r.Entries()[0])
	})
	t.Run("Enabled true", func(t *testing.T) {
		r := migration.NewRegistry()
		result := r.WithOptional(m, true)
		require.Same(t, r, result)
		require.Equal(t, 1, r.Count())
		require.Equal(t, migration.SchemaVersion(1), r.TargetVersion())
		require.Equal(t, m, r.Entries()[0])
	})
}

func TestRegistry_WithOptional_PanicOnMaxMigrations(t *testing.T) {
	r := migration.NewRegistry()
	m := &mockMigration{}

	// Add 64 migrations
	for range 64 {
		r.With(m)
	}

	require.Panics(
		t,
		func() {
			r.WithOptional(m, true)
		},
		"expected panic when exceeding 64 migrations",
	)
}

func TestRegistry_Entries(t *testing.T) {
	r := migration.NewRegistry()
	m0 := &mockMigration{}
	m1 := &mockMigration{}
	m2 := &mockMigration{}

	r.With(m0)
	r.WithOptional(m1, false)
	r.WithOptional(m2, false)

	entries := r.Entries()
	require.Equal(t, 3, len(entries))
	require.Equal(t, m0, entries[0])
	require.Equal(t, m1, entries[1])
	require.Equal(t, m2, entries[2])
}

func TestRegistry_TargetVersion(t *testing.T) {
	r := migration.NewRegistry()
	m0 := &mockMigration{}
	m1 := &mockMigration{}
	m2 := &mockMigration{}

	r.With(m0)
	r.WithOptional(m1, true)
	r.WithOptional(m2, false)

	target := r.TargetVersion()
	require.True(t, target.Has(0))
	require.True(t, target.Has(1))
	require.False(t, target.Has(2))
}

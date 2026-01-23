package migration

// maxMigrations is the maximum number of migrations supported.
// This is limited by SchemaVersion which uses a uint64 bitset (64 bits).
const maxMigrations = 64

// Registry holds all migrations and builds the target version.
type Registry struct {
	entries       [maxMigrations]Migration
	targetVersion SchemaVersion
	nextIndex     uint8 // Next available index for registering a migration
}

// NewRegistry creates a new migration registry.
func NewRegistry() *Registry {
	return &Registry{
		entries:       [maxMigrations]Migration{},
		targetVersion: 0,
		nextIndex:     0,
	}
}

// With adds a mandatory migration to the registry.
// Mandatory migrations are automatically included in targetVersion.
// Returns the registry for method chaining.
func (r *Registry) With(m Migration) *Registry {
	if r.nextIndex >= maxMigrations {
		panic("exceeded maximum number of 64 migrations")
	}
	r.entries[r.nextIndex] = m
	r.targetVersion.Set(r.nextIndex) // Mandatory: always set in target
	r.nextIndex++
	return r
}

// WithOptional adds an optional migration to the registry.
// If enabled is true, the migration is included in the target version.
// Returns the registry for method chaining.
func (r *Registry) WithOptional(m Migration, enabled bool) *Registry {
	if r.nextIndex >= maxMigrations {
		panic("exceeded maximum number of 64 migrations")
	}
	r.entries[r.nextIndex] = m

	if enabled {
		r.targetVersion.Set(r.nextIndex)
	}
	r.nextIndex++
	return r
}

// TargetVersion returns the current target version.
// This is built incrementally as migrations are registered.
func (r *Registry) TargetVersion() SchemaVersion {
	return r.targetVersion
}

// Entries returns all registered migrations.
func (r *Registry) Entries() []Migration {
	return r.entries[:r.nextIndex]
}

// Count returns the number of registered migrations.
func (r *Registry) Count() int {
	return int(r.nextIndex)
}

package db

// Represents a read-only view of the database at a specific point in time.
// If you don't need to read at a specific time, use the database directly.
type Snapshot interface {
	KeyValueReader
	Close() error
}

// Produces a read-only snapshot of the database
type Snapshotter interface {
	NewSnapshot() Snapshot
}

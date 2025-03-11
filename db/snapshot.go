package db

type Snapshot interface {
	KeyValueReader
	Iterable
}

type Snapshotter interface {
	NewSnapshot() Snapshot
}

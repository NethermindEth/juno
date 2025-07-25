package types

type Addr interface {
	~[4]uint64
}

type Hash interface {
	~[4]uint64
}

// Hashable's Hash() is used as ID()
type Hashable[H Hash] interface {
	Hash() H
}

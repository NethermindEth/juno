package trie2

import "errors"

var (
	ErrCommitted  = errors.New("trie is committed")
	ErrEmptyRange = errors.New("empty range")
)

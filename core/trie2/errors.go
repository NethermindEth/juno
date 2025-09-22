package trie2

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/core/felt"
)

var (
	ErrCommitted  = errors.New("trie is committed")
	ErrEmptyRange = errors.New("empty range")
)

type MissingNodeError struct {
	tt    trieutils.TrieType
	owner felt.Felt
	path  trieutils.Path
	hash  felt.Felt
	err   error
}

func (e *MissingNodeError) Error() string {
	if e.owner.Equal(&felt.Zero) {
		return fmt.Sprintf("%s: missing trie node (path %v, hash %v) %v", e.tt, e.path, e.hash, e.err)
	}
	return fmt.Sprintf("%s: missing trie node (owner %v, path %v, hash %v) %v", e.tt, e.owner, e.path, e.hash, e.err)
}

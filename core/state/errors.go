package state

import (
	"errors"

	"github.com/NethermindEth/juno/core"
)

var (
	// ErrContractNotDeployed is the same sentinel as core.ErrContractNotDeployed so that
	// packages that cannot import core/state (e.g. core itself) can still use errors.Is against
	// core.ErrContractNotDeployed while packages that import core/state can use this alias.
	ErrContractNotDeployed        = core.ErrContractNotDeployed
	ErrContractAlreadyDeployed    = errors.New("contract already deployed")
	ErrNoHistoryValue             = errors.New("no history value found")
	ErrHistoricalTrieNotSupported = errors.New("cannot support historical trie")
)

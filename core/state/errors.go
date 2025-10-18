package state

import (
	"errors"
)

var (
	ErrContractNotDeployed        = errors.New("contract not deployed")
	ErrContractAlreadyDeployed    = errors.New("contract already deployed")
	ErrNoHistoryValue             = errors.New("no history value found")
	ErrCheckHeadState             = errors.New("check head state")
	ErrHistoricalTrieNotSupported = errors.New("cannot support historical trie")
)

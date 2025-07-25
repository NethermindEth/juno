package state

import (
	"errors"
)

var (
	ErrContractNotDeployed        = errors.New("contract not deployed")
	ErrContractAlreadyDeployed    = errors.New("contract already deployed")
	ErrNoHistoryValue             = errors.New("no history value found")
	ErrHistoricalTrieNotSupported = errors.New("cannot support historical trie")
)

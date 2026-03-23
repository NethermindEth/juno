package state

import (
	"errors"
)

var (
	ErrContractAlreadyDeployed    = errors.New("contract already deployed")
	ErrNoHistoryValue             = errors.New("no history value found")
	ErrHistoricalTrieNotSupported = errors.New("cannot support historical trie")
)

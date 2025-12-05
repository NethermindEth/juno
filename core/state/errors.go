package state

import (
	"errors"
)

var (
	ErrContractAlreadyDeployed    = errors.New("contract already deployed")
	ErrNoHistoryValue             = errors.New("no history value found")
	ErrCheckHeadState             = errors.New("check head state")
	ErrHistoricalTrieNotSupported = errors.New("cannot support historical trie")
)

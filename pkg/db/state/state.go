package state

import (
	"errors"
	"github.com/NethermindEth/juno/pkg/db"
)

var (
	DbError                = errors.New("database error")
	InvalidContractAddress = errors.New("invalid contract address")
	UnmarshalError         = errors.New("unmarshal error")
	MarshalError           = errors.New("marshal error")
)

type Manager struct {
	codeDatabase    db.Databaser
	storageDatabase db.BlockSpecificDatabase
}

func NewStateManager(codeDatabase db.Databaser, storageDatabase db.BlockSpecificDatabase) *Manager {
	return &Manager{codeDatabase, storageDatabase}
}

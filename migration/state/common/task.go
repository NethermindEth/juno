package common

import "github.com/NethermindEth/juno/db"

type Task struct {
	Batch          db.Batch
	CompletedAddrs int
	EntryCount     int
}

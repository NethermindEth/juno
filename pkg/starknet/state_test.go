package starknet_test

import (
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/pkg/starknet"
	"testing"
)

func TestSynchronizer(t *testing.T) {
	database := db.NewKeyValueDb(t.TempDir(), 0)
	sync := starknet.NewSynchronizer(database)

	err := sync.UpdateState()
	if err != nil {
		return
	}

}

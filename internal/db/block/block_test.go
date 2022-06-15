package block

import (
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/types"
)

func TestManager(t *testing.T) {
	blocks := []*types.Block{
		{
			BlockHash:    types.HexToBlockHash("43950c9e3565cba1f2627b219d4863380f93a8548818ce26019d1bd5eebb0fb"),
			ParentHash:   types.HexToBlockHash("f8fe26de3ce9ee4d543b1152deb2ce549e589524d79598227761d6006b74a9"),
			BlockNumber:  2175,
			Status:       types.BlockStatusAcceptedOnL2,
			Sequencer:    types.HexToAddress("0"),
			NewRoot:      types.HexToFelt("6a42d697b5b735eef03bb71841ed5099d57088f7b5eec8e356fe2601d5ba08f"),
			OldRoot:      types.HexToFelt("1d932dcf7da6c4f7605117cf514d953147161ab2d8f762dcebbb6dad427e519"),
			AcceptedTime: 1652492749,
			TimeStamp:    1652488132,
			TxCount:      2,
			TxCommitment: types.HexToFelt("0"),
			TxHashes: []types.TransactionHash{
				types.HexToTransactionHash("5ce76214481ebb29f912cb5d31abdff34fd42217f5ece9dda76d9fcfd62dc73"),
				types.HexToTransactionHash("4ff16b7673da1f4c4b114d28e0e1a366bd61b702eca3e21882da6c8939e60a2"),
			},
			EventCount:      19,
			EventCommitment: types.HexToFelt("0"),
		},
	}
	manager := NewManager(db.NewKeyValueDb(t.TempDir(), 0))
	for _, block := range blocks {
		key := block.BlockHash
		manager.PutBlock(key, block)
		// Get block by hash
		returnedBlock := manager.GetBlockByHash(key)
		if returnedBlock == nil {
			t.Errorf("unexpected nil after search for block with hash %s", block.BlockHash)
		}
		if !reflect.DeepEqual(block, returnedBlock) {
			t.Errorf("block")
		}
		// Get block by number
		returnedBlock = manager.GetBlockByNumber(block.BlockNumber)
		if returnedBlock == nil {
			t.Errorf("unexpected nil after search for block with number %d", block.BlockNumber)
		}
		if !reflect.DeepEqual(block, returnedBlock) {
			t.Errorf("block")
		}
	}
	manager.Close()
}

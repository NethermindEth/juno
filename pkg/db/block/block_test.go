package block

import (
	"fmt"
	"github.com/NethermindEth/juno/pkg/db"
	"math/big"
	"testing"
)

func TestManager_PutBlock(t *testing.T) {
	blocks := []BlockData{
		{
			BlockProps: BlockProps{
				BlockHash:             fromHexString("43950c9e3565cba1f2627b219d4863380f93a8548818ce26019d1bd5eebb0fb"),
				BlockNumber:           2175,
				ParentBlockHash:       fromHexString("f8fe26de3ce9ee4d543b1152deb2ce549e589524d79598227761d6006b74a9"),
				Status:                "ACCEPTED_ON_L2",
				SequencerAddress:      fromHexString("0"),
				GlobalStateRoot:       fromHexString("06a42d697b5b735eef03bb71841ed5099d57088f7b5eec8e356fe2601d5ba08f"),
				OldRoot:               fromHexString("01d932dcf7da6c4f7605117cf514d953147161ab2d8f762dcebbb6dad427e519"),
				AcceptedTime:          1652492749,
				BlockTimestamp:        1652488132,
				TransactionCount:      2,
				TransactionCommitment: fromHexString("0"),
				EventCount:            19,
				EventCommitment:       fromHexString("0"),
			},
			TransactionsHashes: []big.Int{
				fromHexString("5ce76214481ebb29f912cb5d31abdff34fd42217f5ece9dda76d9fcfd62dc73"),
				fromHexString("4ff16b7673da1f4c4b114d28e0e1a366bd61b702eca3e21882da6c8939e60a2"),
			},
		},
	}
	manager := NewManager(db.New(t.TempDir(), 0))
	for _, block := range blocks {
		key := block.BlockHash
		manager.PutBlock(key, block)
		returnedBlock := manager.GetBlock(key)
		if returnedBlock == nil {
			t.Errorf("unexpected nil after search for block with hash %s", block.BlockHash.Text(16))
		}
		if !equalData(block, *returnedBlock) {
			t.Errorf("block")
		}
	}
}

func equalData(a, b BlockData) bool {
	if !equalProps(a.BlockProps, b.BlockProps) {
		return false
	}
	if len(a.TransactionsHashes) != len(b.TransactionsHashes) {
		return false
	}
	for i, item := range a.TransactionsHashes {
		if item.Cmp(&b.TransactionsHashes[i]) != 0 {
			return false
		}
	}
	return true
}

func equalProps(a, b BlockProps) bool {
	return equalBigInt(a.BlockHash, b.BlockHash) &&
		a.BlockNumber == b.BlockNumber &&
		equalBigInt(a.ParentBlockHash, b.ParentBlockHash) &&
		a.Status == b.Status &&
		equalBigInt(a.SequencerAddress, b.SequencerAddress) &&
		equalBigInt(a.GlobalStateRoot, b.GlobalStateRoot) &&
		equalBigInt(a.OldRoot, b.OldRoot) &&
		a.AcceptedTime == b.AcceptedTime &&
		a.BlockTimestamp == b.BlockTimestamp &&
		a.TransactionCount == b.TransactionCount &&
		equalBigInt(a.TransactionCommitment, b.TransactionCommitment) &&
		a.EventCount == b.EventCount &&
		equalBigInt(a.EventCommitment, b.EventCommitment)
}

func equalBigInt(a, b big.Int) bool {
	return a.Cmp(&b) == 0
}

func fromHexString(s string) big.Int {
	x, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic(any(fmt.Sprintf("invalid string hex: %s", s)))
	}
	return *x
}

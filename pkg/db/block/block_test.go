package block

import (
	"bytes"
	"github.com/NethermindEth/juno/pkg/db"
	"testing"
)

func TestBlockHashKey_Marshal(t *testing.T) {
	var tests = [...]struct {
		Hash      BlockHashKey
		Want      []byte
		WithError bool
	}{
		{
			Hash:      "0x2a",
			Want:      nil,
			WithError: true,
		},
		{
			Hash:      "1",
			Want:      []byte{1},
			WithError: false,
		},
		{
			Hash:      "16fff86123",
			Want:      []byte{22, 255, 248, 97, 35},
			WithError: false,
		},
		{
			Hash:      "43950c9e3565cba1f2627b219d4863380f93a8548818ce26019d1bd5eebb0fb",
			Want:      []byte{4, 57, 80, 201, 227, 86, 92, 186, 31, 38, 39, 178, 25, 212, 134, 51, 128, 249, 58, 133, 72, 129, 140, 226, 96, 25, 209, 189, 94, 235, 176, 251},
			WithError: false,
		},
	}
	for _, test := range tests {
		result, err := test.Hash.Marshal()
		if err != nil {
			if !test.WithError {
				t.Errorf("unexpected error %s", err.Error())
			}
			continue
		}
		if err == nil && test.WithError {
			t.Errorf("expected error")
		}
		if bytes.Compare(result, test.Want) != 0 {
			t.Errorf("Marshal(%s) = %s, want: %s", test.Hash, result, test.Want)
		}
	}
}

func TestManager_PutBlock(t *testing.T) {
	blocks := []BlockValue{
		{
			BlockProps: BlockProps{
				BlockHash:             "43950c9e3565cba1f2627b219d4863380f93a8548818ce26019d1bd5eebb0fb",
				BlockNumber:           2175,
				ParentBlockHash:       "f8fe26de3ce9ee4d543b1152deb2ce549e589524d79598227761d6006b74a9",
				Status:                "ACCEPTED_ON_L2",
				Sequencer:             "",
				GlobalStateRoot:       "06a42d697b5b735eef03bb71841ed5099d57088f7b5eec8e356fe2601d5ba08f",
				OldRoot:               "01d932dcf7da6c4f7605117cf514d953147161ab2d8f762dcebbb6dad427e519",
				AcceptedTime:          1652492749,
				BlockTimestamp:        1652488132,
				TransactionCount:      2,
				TransactionCommitment: "",
				EventCount:            19,
				EventCommitment:       "",
			},
			TransactionsHashes: []string{
				"5ce76214481ebb29f912cb5d31abdff34fd42217f5ece9dda76d9fcfd62dc73",
				"4ff16b7673da1f4c4b114d28e0e1a366bd61b702eca3e21882da6c8939e60a2",
			},
		},
	}
	manager := NewManager(db.New(t.TempDir(), 0))
	for _, block := range blocks {
		key := BlockHashKey(block.BlockHash)
		manager.PutBlock(key, block)
		returnedBlock := manager.GetBlock(key)
		if returnedBlock == nil {
			t.Errorf("unexpected nil after search for block with hash %s", block.BlockHash)
		}
		if !equals(block, *returnedBlock) {
			t.Errorf("block")
		}
	}
}

func equals(a, b BlockValue) bool {
	if a.BlockProps != b.BlockProps {
		return false
	}
	if len(a.TransactionsHashes) != len(b.TransactionsHashes) {
		return false
	}
	for i, item := range a.TransactionsHashes {
		if item != b.TransactionsHashes[i] {
			return false
		}
	}
	return true
}

package state

import (
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/collections"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/trie"
)

func TestManager_TrieNode(t *testing.T) {
	nodes := []trie.TrieNode{
		&trie.BinaryNode{
			LeftH:  new(felt.Felt).SetHex("0x2f50710449a06a9fa789b3c029a63bd0b1f722f46505828a9f815cf91b31d8"),
			RightH: new(felt.Felt).SetHex("0x2f50710449a06a9fa789b3c029a63bd0b1f722f46505828a9f815cf91b31d8"),
		},
		trie.NewEdgeNode(
			collections.NewBitSet(10, []byte{0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03, 0x03}),
			new(felt.Felt).SetHex("0x2f50710449a06a9fa789b3c029a63bd0b1f722f46505828a9f815cf91b31d8"),
		),
	}

	// Init state manager
	env, err := db.NewMDBXEnv(t.TempDir(), 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	stateDb, err := db.NewMDBXDatabase(env, "STATE")
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(stateDb)
	defer manager.Close()

	for _, n := range nodes {
		if err := manager.StoreTrieNode(n); err != nil {
			t.Error(err)
		}
		if node, err := manager.GetTrieNode(n.Hash()); err != nil {
			t.Error(err)
		} else if !n.Hash().Equal(node.Hash()) {
			t.Error("TrieNode are different after Put-Get operation")
		}
	}
}

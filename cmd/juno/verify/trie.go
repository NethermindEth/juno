package verify

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/db"
)

type TrieType string

const (
	TrieTypeState    TrieType = "state"
	TrieTypeClass    TrieType = "class"
	TrieTypeContract TrieType = "contract"
)

type TrieConfig struct {
	Tries []TrieType
}

type TrieVerifier struct {
	database db.KeyValueStore
}

func NewTrieVerifier(database db.KeyValueStore) *TrieVerifier {
	return &TrieVerifier{
		database: database,
	}
}

// Name returns the name of this verifier.
func (v *TrieVerifier) Name() string {
	return "trie"
}

// DefaultConfig returns the default configuration (verify all tries).
func (v *TrieVerifier) DefaultConfig() Config {
	return &TrieConfig{
		Tries: nil, // nil = all tries
	}
}

// Run executes the trie verification.
func (v *TrieVerifier) Run(ctx context.Context, cfg Config) error {
	trieCfg, ok := cfg.(*TrieConfig)
	if !ok {
		return fmt.Errorf("invalid config type for trie verifier: expected *TrieConfig")
	}

	// If Tries is empty, verify all tries
	typesToVerify := trieCfg.Tries
	if len(typesToVerify) == 0 {
		typesToVerify = []TrieType{TrieTypeState, TrieTypeClass, TrieTypeContract}
	}

	for _, trieType := range typesToVerify {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := v.verifyTrie(ctx, trieType); err != nil {
			return fmt.Errorf("trie type %s: %w", trieType, err)
		}
	}

	return nil
}

// verifyTrie verifies a specific trie type.
func (v *TrieVerifier) verifyTrie(ctx context.Context, trieType TrieType) error {
	// TODO: Implement actual trie verification logic
	// This is a stub implementation that demonstrates the structure.
	// The actual implementation should:
	// 1. Collect leaf nodes from the specified trie
	// 2. Rebuild the trie in memory from leaves
	// 3. Calculate the root hash
	// 4. Compare with stored root hash

	switch trieType {
	case TrieTypeState:
		// Verify StateTrie
		return nil
	case TrieTypeClass:
		// Verify ClassesTrie
		return nil
	case TrieTypeContract:
		// Verify all contract storage tries
		return nil
	default:
		return fmt.Errorf("unknown trie type: %s", trieType)
	}
}

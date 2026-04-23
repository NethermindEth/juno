package trie

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
)

var ErrCorruptionDetected = errors.New("corruption detected")

func VerifyTrie(
	ctx context.Context,
	reader *trie.ReadStorage,
	height uint8,
	hashFn crypto.HashFn,
	expectedRoot *felt.Felt,
) error {
	rootKey, err := reader.RootKey()
	if err != nil {
		return fmt.Errorf("failed to get root key: %w", err)
	}

	if rootKey == nil {
		return nil
	}

	rootHash, err := verifyNode(ctx, reader, rootKey, nil, height, hashFn)
	if err != nil {
		return err
	}

	if rootHash.Cmp(expectedRoot) != 0 {
		return fmt.Errorf(
			"%w: root hash mismatch, expected %s, got %s",
			ErrCorruptionDetected, expectedRoot.String(), rootHash.String(),
		)
	}

	return nil
}

func verifyNode(
	ctx context.Context,
	reader *trie.ReadStorage,
	key *trie.BitArray,
	parentKey *trie.BitArray,
	height uint8,
	hashFn crypto.HashFn,
) (felt.Felt, error) {
	if err := ctx.Err(); err != nil {
		return felt.Zero, err
	}

	node, err := reader.Get(key)
	if err != nil {
		return felt.Zero, fmt.Errorf("failed to get node at key %s: %w", key.String(), err)
	}

	p := path(key, parentKey)

	if key.Len() == height {
		return node.Hash(&p, hashFn), nil
	}

	leftFn := func(ctx context.Context) (felt.Felt, error) {
		if node.Left.IsEmpty() {
			return felt.Zero, nil
		}
		return verifyNode(ctx, reader, node.Left, key, height, hashFn)
	}

	rightFn := func(ctx context.Context) (felt.Felt, error) {
		if node.Right.IsEmpty() {
			return felt.Zero, nil
		}
		return verifyNode(ctx, reader, node.Right, key, height, hashFn)
	}

	leftHash, rightHash, err := TraverseBinary(ctx, key.Len(), ConcurrencyMaxDepth, leftFn, rightFn)
	if err != nil {
		return felt.Zero, err
	}

	recomputed := hashFn(&leftHash, &rightHash)
	if recomputed.Cmp(node.Value) != 0 {
		return felt.Zero, fmt.Errorf(
			"%w: node at key %s, stored hash=%s, recomputed hash=%s",
			ErrCorruptionDetected, key.String(), node.Value.String(), recomputed.String(),
		)
	}

	return node.Hash(&p, hashFn), nil
}

func path(key, parentKey *trie.BitArray) trie.BitArray {
	if parentKey == nil {
		return key.Copy()
	}

	var pathKey trie.BitArray
	pathKey.LSBs(key, parentKey.Len()+1)
	return pathKey
}

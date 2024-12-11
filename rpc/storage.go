package rpc

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
)

/****************************************************
		Storage Handlers
*****************************************************/

// StorageAt gets the value of the storage at the given address and key.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L110
func (h *Handler) StorageAt(address, key felt.Felt, id BlockID) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getStorageAt")

	// This checks if the contract exists because if a key doesn't exist in contract storage,
	// the returned value is always zero and error is nil.
	_, err := stateReader.ContractClassHash(&address)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, ErrContractNotFound
		}
		h.log.Errorw("Failed to get contract nonce", "err", err)
		return nil, ErrInternal
	}

	value, err := stateReader.ContractStorage(&address, &key)
	if err != nil {
		return nil, ErrContractNotFound
	}

	return value, nil
}

// StorageProof returns the merkle paths in one of the state tries: global state, classes, individual contract
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/647caa00c0223e1daab1b2f3acc4e613ba2138aa/api/starknet_api_openrpc.json#L910
func (h *Handler) StorageProof(
	id BlockID,
	classes, contracts []felt.Felt,
	storageKeys []StorageKeys,
) (*StorageProofResult, *jsonrpc.Error) {
	if !id.Latest {
		return nil, ErrStorageProofNotSupported
	}

	stateReader, stateCloser, err := h.bcReader.HeadState()
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getStorageProof")

	head, err := h.bcReader.Head()
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	trieReader, stateCloser2, err := h.bcReader.HeadTrie()
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}
	defer h.callAndLogErr(stateCloser2, "Error closing trie reader in getStorageProof")

	storageRoot, classRoot, err := trieReader.StateAndClassRoot()
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	result := &StorageProofResult{
		GlobalRoots: &GlobalRoots{
			ContractsTreeRoot: storageRoot,
			ClassesTreeRoot:   classRoot,
			BlockHash:         head.Hash,
		},
	}

	result.ClassesProof, err = getClassesProof(trieReader, classes)
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}
	result.ContractsProof, err = getContractsProof(stateReader, trieReader, contracts)
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}
	result.ContractsStorageProofs, err = getContractsStorageProofs(trieReader, storageKeys)
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	return result, nil
}

// StorageKeys represents an item in `contracts_storage_keys. parameter
// https://github.com/starkware-libs/starknet-specs/blob/647caa00c0223e1daab1b2f3acc4e613ba2138aa/api/starknet_api_openrpc.json#L938
type StorageKeys struct {
	Contract felt.Felt   `json:"contract_address"`
	Keys     []felt.Felt `json:"storage_keys"`
}

// MerkleNode represents a proof node in a trie
// https://github.com/starkware-libs/starknet-specs/blob/647caa00c0223e1daab1b2f3acc4e613ba2138aa/api/starknet_api_openrpc.json#L3632
// Implemented by MerkleBinaryNode, MerkleEdgeNode
type MerkleNode interface {
	AsProofNode() trie.ProofNode
}

// https://github.com/starkware-libs/starknet-specs/blob/647caa00c0223e1daab1b2f3acc4e613ba2138aa/api/starknet_api_openrpc.json#L3644
type MerkleBinaryNode struct {
	Left  *felt.Felt `json:"left"`
	Right *felt.Felt `json:"right"`
}

func (mbn *MerkleBinaryNode) AsProofNode() trie.ProofNode {
	return &trie.Binary{
		LeftHash:  mbn.Left,
		RightHash: mbn.Right,
	}
}

// https://github.com/starkware-libs/starknet-specs/blob/8cf463b79ba1dd876f67c7f637e5ea48beb07b5b/api/starknet_api_openrpc.json#L3720
type MerkleEdgeNode struct {
	Path   string     `json:"path"`
	Length int        `json:"length"`
	Child  *felt.Felt `json:"child"`
}

func (men *MerkleEdgeNode) AsProofNode() trie.ProofNode {
	f, _ := new(felt.Felt).SetString(men.Path)
	pbs := f.Bytes()
	path := trie.NewKey(uint8(men.Length), pbs[:])

	return &trie.Edge{
		Path:  &path,
		Child: men.Child,
	}
}

// HashToNode represents an item in `NODE_HASH_TO_NODE_MAPPING` specified here
// https://github.com/starkware-libs/starknet-specs/blob/647caa00c0223e1daab1b2f3acc4e613ba2138aa/api/starknet_api_openrpc.json#L3667
type HashToNode struct {
	Hash *felt.Felt `json:"node_hash"`
	Node MerkleNode `json:"node"`
}

// https://github.com/starkware-libs/starknet-specs/blob/8cf463b79ba1dd876f67c7f637e5ea48beb07b5b/api/starknet_api_openrpc.json#L986
type LeafData struct {
	Nonce     *felt.Felt `json:"nonce"`
	ClassHash *felt.Felt `json:"class_hash"`
}

// https://github.com/starkware-libs/starknet-specs/blob/8cf463b79ba1dd876f67c7f637e5ea48beb07b5b/api/starknet_api_openrpc.json#L979
type ContractProof struct {
	Nodes      []*HashToNode `json:"nodes"`
	LeavesData []*LeafData   `json:"contract_leaves_data"`
}

// https://github.com/starkware-libs/starknet-specs/blob/8cf463b79ba1dd876f67c7f637e5ea48beb07b5b/api/starknet_api_openrpc.json#L1011
type GlobalRoots struct {
	ContractsTreeRoot *felt.Felt `json:"contracts_tree_root"`
	ClassesTreeRoot   *felt.Felt `json:"classes_tree_root"`
	BlockHash         *felt.Felt `json:"block_hash"`
}

// https://github.com/starkware-libs/starknet-specs/blob/8cf463b79ba1dd876f67c7f637e5ea48beb07b5b/api/starknet_api_openrpc.json#L970
type StorageProofResult struct {
	ClassesProof           []*HashToNode   `json:"classes_proof"`
	ContractsProof         *ContractProof  `json:"contracts_proof"`
	ContractsStorageProofs [][]*HashToNode `json:"contracts_storage_proofs"`
	GlobalRoots            *GlobalRoots    `json:"global_roots"`
}

func getClassesProof(reader core.TrieReader, classes []felt.Felt) ([]*HashToNode, error) {
	cTrie, _, err := reader.ClassTrie()
	if err != nil {
		return nil, err
	}
	result := []*HashToNode{}
	for _, class := range utils.Unique(classes) {
		nodes, err := getProof(cTrie, &class)
		if err != nil {
			return nil, err
		}
		result = append(result, nodes...)
	}

	return deduplicate(result), nil
}

func getContractsProof(stReader core.StateReader, trReader core.TrieReader, contracts []felt.Felt) (*ContractProof, error) {
	sTrie, _, err := trReader.StorageTrie()
	if err != nil {
		return nil, err
	}

	result := &ContractProof{
		Nodes:      []*HashToNode{},
		LeavesData: make([]*LeafData, 0, len(contracts)),
	}

	for _, contract := range contracts {
		leafData, err := getLeafData(stReader, &contract)
		if err != nil {
			return nil, err
		}
		result.LeavesData = append(result.LeavesData, leafData)

		nodes, err := getProof(sTrie, &contract)
		if err != nil {
			return nil, err
		}
		result.Nodes = append(result.Nodes, nodes...)
	}

	result.Nodes = deduplicate(result.Nodes)
	return result, nil
}

func getLeafData(reader core.StateReader, contract *felt.Felt) (*LeafData, error) {
	nonce, err := reader.ContractNonce(contract)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	classHash, err := reader.ContractClassHash(contract)
	if err != nil {
		return nil, err
	}

	return &LeafData{
		Nonce:     nonce,
		ClassHash: classHash,
	}, nil
}

func getContractsStorageProofs(reader core.TrieReader, keys []StorageKeys) ([][]*HashToNode, error) {
	result := make([][]*HashToNode, 0, len(keys))

	for _, key := range keys {
		csTrie, err := reader.StorageTrieForAddr(&key.Contract)
		if err != nil {
			// Note: if contract does not exist, `StorageTrieForAddr()` returns an empty trie, not an error
			return nil, err
		}

		nodes := []*HashToNode{}
		for _, slot := range utils.Unique(key.Keys) {
			proof, err := getProof(csTrie, &slot)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, proof...)
		}
		result = append(result, deduplicate(nodes))
	}

	return result, nil
}

func getProof(t *trie.Trie, elt *felt.Felt) ([]*HashToNode, error) {
	feltBytes := elt.Bytes()
	key := trie.NewKey(core.ContractStorageTrieHeight, feltBytes[:])
	nodes, err := trie.GetProof(&key, t)
	if err != nil {
		return nil, err
	}

	// adapt proofs to the expected format
	hashNodes := make([]*HashToNode, len(nodes))
	for i, node := range nodes {
		var merkle MerkleNode

		if binary, ok := node.(*trie.Binary); ok {
			merkle = &MerkleBinaryNode{
				Left:  binary.LeftHash,
				Right: binary.RightHash,
			}
		}
		if edge, ok := node.(*trie.Edge); ok {
			path := edge.Path
			f := path.Felt()
			merkle = &MerkleEdgeNode{
				Path:   f.String(),
				Length: int(edge.Len()),
				Child:  edge.Child,
			}
		}

		hashNodes[i] = &HashToNode{
			Hash: node.Hash(t.HashFunc()),
			Node: merkle,
		}
	}

	return hashNodes, nil
}

func deduplicate(proof []*HashToNode) []*HashToNode {
	if len(proof) == 0 {
		return proof
	}

	keyF := func(node *HashToNode) felt.Felt { return *node.Hash }
	return utils.UniqueFunc(proof, keyF)
}

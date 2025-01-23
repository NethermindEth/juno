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
		Contract Handlers
*****************************************************/

// Nonce returns the nonce associated with the given address in the given block number
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L633
func (h *Handler) Nonce(id BlockID, address felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getNonce")

	nonce, err := stateReader.ContractNonce(&address)
	if err != nil {
		return nil, ErrContractNotFound
	}

	return nonce, nil
}

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

type StorageProofResult struct {
	ClassesProof           []*HashToNode   `json:"classes_proof"`
	ContractsProof         *ContractProof  `json:"contracts_proof"`
	ContractsStorageProofs [][]*HashToNode `json:"contracts_storage_proofs"`
	GlobalRoots            *GlobalRoots    `json:"global_roots"`
}

func (h *Handler) StorageProof(id BlockID,
	classes, contracts []felt.Felt, storageKeys []StorageKeys,
) (*StorageProofResult, *jsonrpc.Error) {
	// We do not support historical storage proofs for now
	if !id.Latest {
		return nil, ErrStorageProofNotSupported
	}

	head, err := h.bcReader.Head()
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	state, closer, err := h.bcReader.HeadState()
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}
	defer h.callAndLogErr(closer, "Error closing state reader in getStorageProof")

	classTrie, err := state.ClassTrie()
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	contractTrie, err := state.ContractTrie()
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	// Do a sanity check and remove duplicates from the keys
	classes = utils.Set(classes)
	contracts = utils.Set(contracts)

	// Remove duplicates from the storage keys
	mergedStorageKeys := make(map[felt.Felt][]felt.Felt)
	for _, storageKey := range storageKeys {
		if existing, ok := mergedStorageKeys[storageKey.Contract]; ok {
			mergedStorageKeys[storageKey.Contract] = append(existing, storageKey.Keys...)
		} else {
			mergedStorageKeys[storageKey.Contract] = storageKey.Keys
		}
	}

	uniqueStorageKeys := make([]StorageKeys, 0, len(mergedStorageKeys))
	for contract, keys := range mergedStorageKeys {
		uniqueStorageKeys = append(uniqueStorageKeys, StorageKeys{Contract: contract, Keys: utils.Set(keys)})
	}

	classProof, err := getClassProof(classTrie, classes)
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	contractProof, err := getContractProof(contractTrie, state, contracts)
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	contractStorageProof, err := getContractStorageProof(state, uniqueStorageKeys)
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	contractTreeRoot, err := contractTrie.Root()
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	classTreeRoot, err := classTrie.Root()
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	return &StorageProofResult{
		ClassesProof:           classProof,
		ContractsProof:         contractProof,
		ContractsStorageProofs: contractStorageProof,
		GlobalRoots: &GlobalRoots{
			ContractsTreeRoot: contractTreeRoot,
			ClassesTreeRoot:   classTreeRoot,
			BlockHash:         head.Hash,
		},
	}, nil
}

func getClassProof(tr *trie.Trie, classes []felt.Felt) ([]*HashToNode, error) {
	classProof := trie.NewProofNodeSet()
	for _, class := range classes {
		if err := tr.Prove(&class, classProof); err != nil {
			return nil, err
		}
	}

	return adaptProofNodes(classProof), nil
}

func getContractProof(tr *trie.Trie, state core.StateReader, contracts []felt.Felt) (*ContractProof, error) {
	contractProof := trie.NewProofNodeSet()
	contractLeavesData := make([]*LeafData, len(contracts))
	for i, contract := range contracts {
		if err := tr.Prove(&contract, contractProof); err != nil {
			return nil, err
		}

		root, err := tr.Root()
		if err != nil {
			return nil, err
		}

		nonce, err := state.ContractNonce(&contract)
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) { // contract does not exist, skip getting leaf data
				continue
			}
			return nil, err
		}

		classHash, err := state.ContractClassHash(&contract)
		if err != nil {
			return nil, err
		}

		contractLeavesData[i] = &LeafData{
			Nonce:       nonce,
			ClassHash:   classHash,
			StorageRoot: root,
		}
	}

	return &ContractProof{
		Nodes:      adaptProofNodes(contractProof),
		LeavesData: contractLeavesData,
	}, nil
}

func getContractStorageProof(state core.StateReader, storageKeys []StorageKeys) ([][]*HashToNode, error) {
	contractStorageRes := make([][]*HashToNode, len(storageKeys))
	for i, storageKey := range storageKeys {
		contractStorageTrie, err := state.ContractStorageTrie(&storageKey.Contract)
		if err != nil {
			return nil, err
		}

		contractStorageProof := trie.NewProofNodeSet()
		for _, key := range storageKey.Keys {
			if err := contractStorageTrie.Prove(&key, contractStorageProof); err != nil {
				return nil, err
			}
		}

		contractStorageRes[i] = adaptProofNodes(contractStorageProof)
	}

	return contractStorageRes, nil
}

func adaptProofNodes(proof *trie.ProofNodeSet) []*HashToNode {
	nodes := make([]*HashToNode, proof.Size())
	nodeList := proof.List()
	for i, hash := range proof.Keys() {
		var node Node

		switch n := nodeList[i].(type) {
		case *trie.Binary:
			node = &BinaryNode{
				Left:  n.LeftHash,
				Right: n.RightHash,
			}
		case *trie.Edge:
			path := n.Path.Felt()
			node = &EdgeNode{
				Path:   path.String(),
				Length: int(n.Path.Len()),
				Child:  n.Child,
			}
		}

		nodes[i] = &HashToNode{
			Hash: &hash,
			Node: node,
		}
	}

	return nodes
}

type StorageKeys struct {
	Contract felt.Felt   `json:"contract_address"`
	Keys     []felt.Felt `json:"storage_keys"`
}

type Node interface {
	AsProofNode() trie.ProofNode
}

type BinaryNode struct {
	Left  *felt.Felt `json:"left"`
	Right *felt.Felt `json:"right"`
}

type EdgeNode struct {
	Path   string     `json:"path"`
	Length int        `json:"length"`
	Child  *felt.Felt `json:"child"`
}

func (e *EdgeNode) AsProofNode() trie.ProofNode {
	f, _ := new(felt.Felt).SetString(e.Path)
	pbs := f.Bytes()

	return &trie.Edge{
		Path:  new(trie.BitArray).SetBytes(uint8(e.Length), pbs[:]),
		Child: e.Child,
	}
}

func (b *BinaryNode) AsProofNode() trie.ProofNode {
	return &trie.Binary{
		LeftHash:  b.Left,
		RightHash: b.Right,
	}
}

type HashToNode struct {
	Hash *felt.Felt `json:"node_hash"`
	Node Node       `json:"node"`
}

type LeafData struct {
	Nonce       *felt.Felt `json:"nonce"`
	ClassHash   *felt.Felt `json:"class_hash"`
	StorageRoot *felt.Felt `json:"storage_root"`
}

type ContractProof struct {
	Nodes      []*HashToNode `json:"nodes"`
	LeavesData []*LeafData   `json:"contract_leaves_data"`
}

type GlobalRoots struct {
	ContractsTreeRoot *felt.Felt `json:"contracts_tree_root"`
	ClassesTreeRoot   *felt.Felt `json:"classes_tree_root"`
	BlockHash         *felt.Felt `json:"block_hash"`
}

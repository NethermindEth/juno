package rpcv8

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils"
)

const (
	MissingContractAddress = "missing field: contract_address"
	MissingStorageKeys     = "missing field: storage_keys"
)

/****************************************************
		Contract Handlers
*****************************************************/

// StorageAt gets the value of the storage at the given address and key.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L110
func (h *Handler) StorageAt(address, key *felt.Felt, id *BlockID) (*felt.Felt, *jsonrpc.Error) {
	stateReader, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	// This checks if the contract exists because if a key doesn't exist in contract storage,
	// the returned value is always zero and error is nil.
	_, err := stateReader.ContractClassHash(address)
	if err != nil {
		if errors.Is(err, state.ErrContractNotDeployed) {
			return nil, rpccore.ErrContractNotFound
		}
		h.log.Errorw("Failed to get contract nonce", "err", err)
		return nil, rpccore.ErrInternal
	}

	value, err := stateReader.ContractStorage(address, key)
	if err != nil {
		return nil, rpccore.ErrInternal
	}

	return &value, nil
}

type StorageProofResult struct {
	ClassesProof           []*HashToNode   `json:"classes_proof"`
	ContractsProof         *ContractProof  `json:"contracts_proof"`
	ContractsStorageProofs [][]*HashToNode `json:"contracts_storage_proofs"`
	GlobalRoots            *GlobalRoots    `json:"global_roots"`
}

func (h *Handler) StorageProof(
	id *BlockID, classes, contracts []felt.Felt, storageKeys []StorageKeys,
) (*StorageProofResult, *jsonrpc.Error) {
	state, err := h.bcReader.HeadState()
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	chainHeight, err := h.bcReader.Height()
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	// TODO(infrmtcs): This is still a half baked solution because we're using another transaction to get the block number.
	// We don't use the head query directly to avoid race condition where there is a new incoming block.
	// Currently it's still working because we don't have revert yet.
	// We should figure out a way to merge the two transactions.
	head, err := h.bcReader.BlockByNumber(chainHeight)
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	// We do not support historical storage proofs for now
	// Ensure that the block requested is the head block
	if rpcErr := h.isBlockSupported(id, chainHeight); rpcErr != nil {
		return nil, rpcErr
	}

	classTrie, err := state.ClassTrie()
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	contractTrie, err := state.ContractTrie()
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	// Do a sanity check and remove duplicates from the inputs
	classes = utils.Set(classes)
	contracts = utils.Set(contracts)
	uniqueStorageKeys, rpcErr := processStorageKeys(storageKeys)
	if rpcErr != nil {
		return nil, rpcErr
	}

	classProof, err := getClassProof(classTrie, classes)
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	contractProof, err := getContractProof(contractTrie, state, contracts)
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	contractStorageProof, err := getContractStorageProof(state, uniqueStorageKeys)
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	contractTreeRoot := contractTrie.Hash()
	classTreeRoot := classTrie.Hash()

	return &StorageProofResult{
		ClassesProof:           classProof,
		ContractsProof:         contractProof,
		ContractsStorageProofs: contractStorageProof,
		GlobalRoots: &GlobalRoots{
			ContractsTreeRoot: &contractTreeRoot,
			ClassesTreeRoot:   &classTreeRoot,
			BlockHash:         head.Hash,
		},
	}, nil
}

// Ensures each contract is unique and each storage key in each contract is unique
func processStorageKeys(storageKeys []StorageKeys) ([]StorageKeys, *jsonrpc.Error) {
	if len(storageKeys) == 0 {
		return nil, nil
	}

	merged := make(map[felt.Felt][]felt.Felt, len(storageKeys))
	for _, sk := range storageKeys {
		// Ensure that both contract and keys are provided
		if sk.Contract == nil {
			return nil, jsonrpc.Err(jsonrpc.InvalidParams, MissingContractAddress)
		}
		if len(sk.Keys) == 0 {
			return nil, jsonrpc.Err(jsonrpc.InvalidParams, MissingStorageKeys)
		}

		contract := *sk.Contract
		merged[contract] = append(merged[contract], sk.Keys...)
	}

	uniqueStorageKeys := make([]StorageKeys, 0, len(merged))
	for contract, keys := range merged {
		uniqueStorageKeys = append(uniqueStorageKeys, StorageKeys{Contract: &contract, Keys: utils.Set(keys)})
	}

	return uniqueStorageKeys, nil
}

// isBlockSupported checks if the block ID requested is supported for storage proofs
// Currently returns true only if the block ID requested matches the head block
func (h *Handler) isBlockSupported(blockID *BlockID, chainHeight uint64) *jsonrpc.Error {
	var blockNumber uint64
	switch {
	case blockID.IsLatest():
		return nil
	case blockID.IsPending():
		// TODO: Remove this case when specs replaced BLOCK_ID by another type.
		return rpccore.ErrCallOnPending
	case blockID.IsHash():
		header, err := h.bcReader.BlockHeaderByHash(blockID.Hash())
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				return rpccore.ErrBlockNotFound
			}
			return rpccore.ErrInternal.CloneWithData(err)
		}
		blockNumber = header.Number
	case blockID.IsNumber():
		blockNumber = blockID.Number()
	default:
		panic(fmt.Sprintf("invalid block id type %d", blockID.Type()))
	}

	switch {
	case blockNumber < chainHeight:
		return rpccore.ErrStorageProofNotSupported
	case blockNumber > chainHeight:
		return rpccore.ErrBlockNotFound
	}
	return nil
}

func getClassProof(tr *trie2.Trie, classes []felt.Felt) ([]*HashToNode, error) {
	classProof := trie2.NewProofNodeSet()
	for _, class := range classes {
		if err := tr.Prove(&class, classProof); err != nil {
			return nil, err
		}
	}

	return adaptProofNodes(classProof), nil
}

func getContractProof(tr *trie2.Trie, st state.StateReader, contracts []felt.Felt) (*ContractProof, error) {
	contractProof := trie2.NewProofNodeSet()
	contractLeavesData := make([]*LeafData, len(contracts))
	for i, contract := range contracts {
		if err := tr.Prove(&contract, contractProof); err != nil {
			return nil, err
		}

		root := tr.Hash()

		nonce, err := st.ContractNonce(&contract)
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) { // contract does not exist, skip getting leaf data
				continue
			}
			return nil, err
		}

		classHash, err := st.ContractClassHash(&contract)
		if err != nil {
			return nil, err
		}

		contractLeavesData[i] = &LeafData{
			Nonce:       &nonce,
			ClassHash:   &classHash,
			StorageRoot: &root,
		}
	}

	return &ContractProof{
		Nodes:      adaptProofNodes(contractProof),
		LeavesData: contractLeavesData,
	}, nil
}

func getContractStorageProof(state state.StateReader, storageKeys []StorageKeys) ([][]*HashToNode, error) {
	contractStorageRes := make([][]*HashToNode, len(storageKeys))
	for i, storageKey := range storageKeys {
		contractStorageTrie, err := state.ContractStorageTrie(storageKey.Contract)
		if err != nil {
			return nil, err
		}

		contractStorageProof := trie2.NewProofNodeSet()
		for _, key := range storageKey.Keys {
			if err := contractStorageTrie.Prove(&key, contractStorageProof); err != nil {
				return nil, err
			}
		}

		contractStorageRes[i] = adaptProofNodes(contractStorageProof)
	}

	return contractStorageRes, nil
}

func adaptProofNodes(proof *trie2.ProofNodeSet) []*HashToNode {
	nodes := make([]*HashToNode, proof.Size())
	nodeList := proof.List()
	for i, hash := range proof.Keys() {
		var node Node

		switch n := nodeList[i].(type) {
		case *trienode.BinaryNode:
			node = &BinaryNode{
				Left:  nodeFelt(n.Children[0]),
				Right: nodeFelt(n.Children[1]),
			}
		case *trienode.EdgeNode:
			pathFelt := n.Path.Felt()
			node = &EdgeNode{
				Path:   pathFelt.String(),
				Length: int(n.Path.Len()),
				Child:  nodeFelt(n.Child),
			}
		}

		nodes[i] = &HashToNode{
			Hash: &hash,
			Node: node,
		}
	}

	return nodes
}

func nodeFelt(n trienode.Node) *felt.Felt {
	switch n := n.(type) {
	case *trienode.HashNode:
		return (*felt.Felt)(n)
	case *trienode.ValueNode:
		return (*felt.Felt)(n)
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

type StorageKeys struct {
	Contract *felt.Felt  `json:"contract_address"`
	Keys     []felt.Felt `json:"storage_keys"`
}

type Node interface {
	TrieNode() trienode.Node
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

func (e *EdgeNode) TrieNode() trienode.Node {
	f, _ := new(felt.Felt).SetString(e.Path)
	pbs := f.Bytes()

	return &trienode.EdgeNode{
		Path:  new(trie2.Path).SetBytes(uint8(e.Length), pbs[:]),
		Child: (*trienode.HashNode)(e.Child), // TODO(weiihann): this could be a value node too
	}
}

func (b *BinaryNode) TrieNode() trienode.Node {
	return &trienode.BinaryNode{
		// TODO(weiihann): this could be a value node too
		Children: [2]trienode.Node{(*trienode.HashNode)(b.Left), (*trienode.HashNode)(b.Right)},
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

package vmrpc

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/trie"

	statedb "github.com/NethermindEth/juno/internal/db/state"
)

type storageRPCServer struct {
	stateManager *statedb.Manager
	UnimplementedStorageAdapterServer
}

func NewStorageRPCServer(stateManager *statedb.Manager) *storageRPCServer {
	return &storageRPCServer{
		stateManager: stateManager,
	}
}

func (s *storageRPCServer) GetPatriciaNode(ctx context.Context, request *GetValueRequest) (*VMTrieNode, error) {
	node, err := s.stateManager.GetTrieNode(new(felt.Felt).SetBytes(request.GetKey()))
	if err != nil {
		return nil, err
	}
	nodeP := &VMTrieNode{
		Len:    uint32(node.Path().Len()),
		Path:   node.Path().Bytes(),
		Bottom: node.Bottom().ByteSlice(),
	}
	switch n := node.(type) {
	case *trie.EdgeNode:
		nodeP.Type = NodeType_EDGE_NODE
	case *trie.BinaryNode:
		nodeP.Type = NodeType_BINARY_NODE
		nodeP.Left = n.LeftH.ByteSlice()
		nodeP.Right = n.RightH.ByteSlice()
	default:
		return nil, errors.New("unsupported trie node type")
	}
	return nodeP, nil
}

func (s *storageRPCServer) GetContractState(ctx context.Context, request *GetValueRequest) (*VMContractState, error) {
	st, err := s.stateManager.GetContractState(new(felt.Felt).SetBytes(request.GetKey()))
	if err != nil {
		return nil, err
	}
	return &VMContractState{
		ContractHash: st.ContractHash.ByteSlice(),
		StorageRoot:  st.StorageRoot.ByteSlice(),
		Height:       uint32(state.StorageTrieHeight),
	}, nil
}

func (s *storageRPCServer) GetContractDefinition(ctx context.Context, request *GetValueRequest) (*VMContractDefinition, error) {
	cd, err := s.stateManager.GetContract(new(felt.Felt).SetBytes(request.GetKey()))
	if err != nil {
		return nil, err
	}
	var fullDefMap map[string]interface{}
	if err := json.Unmarshal(cd.FullDef, &fullDefMap); err != nil {
		return nil, err
	}

	decodedProgram, err := base64.StdEncoding.DecodeString(fullDefMap["program"].(string))
	if err != nil {
		return nil, err
	}
	gr, err := gzip.NewReader(bytes.NewBuffer(decodedProgram))
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	decodedProgram, err = ioutil.ReadAll(gr)
	if err != nil {
		return nil, err
	}

	fullDefMap["program"] = decodedProgram
	fullDef, err := json.Marshal(fullDefMap)
	if err != nil {
		return nil, err
	}

	return &VMContractDefinition{
		Value: fullDef,
	}, nil
}

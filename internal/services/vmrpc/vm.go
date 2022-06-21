package vmrpc

import (
	"context"
	_ "embed"
)

type storageRPCServer struct {
	UnimplementedStorageAdapterServer
}

// DEBUG.
//go:embed test_contract_def.json
var def string

// DEBUG.
var db = map[string]string{
	"patricia_node:0704dfcbc470377c68e6f5ffb83970ebd0d7c48d5b8d2f4ed61a24e795e034bd": "002e9723e54711aec56e3fb6ad1bb8272f64ec92e0a43a20feed943b1d4f73c5057dde83c18c0efe7123c36a52d704cf27d5c38cdf0b1e1edc3b0dae3ee4e374fb",
	"contract_state:002e9723e54711aec56e3fb6ad1bb8272f64ec92e0a43a20feed943b1d4f73c5": `{
			"storage_commitment_tree": {
				"root": "04fb440e8ca9b74fc12a22ebffe0bc0658206337897226117b985434c239c028", 
				"height": 251
			}, 
			"contract_hash": "050b2148c0d782914e0b12a1a32abe5e398930b7e914f82c65cb7afce0a0ab9b"
		}`,
	"patricia_node:04fb440e8ca9b74fc12a22ebffe0bc0658206337897226117b985434c239c028":            "00000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000084fb",
	"starknet_storage_leaf:0000000000000000000000000000000000000000000000000000000000000003":    "0000000000000000000000000000000000000000000000000000000000000003",
	"contract_definition_fact:050b2148c0d782914e0b12a1a32abe5e398930b7e914f82c65cb7afce0a0ab9b": def,
}

func NewStorageRPCServer() *storageRPCServer {
	return &storageRPCServer{}
}

// GetValue calls the get_value method of the Storage adapter,
// StorageRPCClient, on the Cairo RPC server.
func (s *storageRPCServer) GetValue(ctx context.Context, request *GetValueRequest) (*GetValueResponse, error) {
	// XXX: Handle the following cases i.e. key prefixes. See the
	// following for details https://github.com/starkware-libs/cairo-lang/blob/167b28bcd940fd25ea3816204fa882a0b0a49603/src/starkware/starkware_utils/serializable.py#L25-L31.
	//
	//	1.	patricia_node. A node in a global state or contract storage
	//			trie. Note that the virtual machine does not make a
	//			distinction as to whether the node being queried comes from
	//			the global state trie or contract storage trie. One idea on
	//			how to address this is to query the global state tree and only
	//			if that key does not exist, send a query to the contract
	//			storage trie. See the following for details https://github.com/eqlabs/pathfinder/blob/82425d44d7aa148bd31a60a7823a3e42b8d613f4/py/src/call.py#L338-L353.
	//
	//	2.	contract_state. The key suffix is the result of
	//			h(h(h(contract_hash, storage_root), 0), 0) where h is the
	//			StarkNet Pedersen hash and the value is perhaps better
	// 			explained by the following reference and example:
	//				- https://github.com/eqlabs/pathfinder/blob/31a308709141cc0d0c0f5568a67e2c9aa89be959/py/src/call.py#L355-L380.
	//				- https://github.com/NethermindEth/juno/blob/42077622e5134e6835f05df0fac9dfd0a2505e9f/pkg/rpc/call.py#L27-L31.
	//
	//	3.	contract_definition_fact. The key suffix is the contract hash
	//			(class hash) and the value is the compiled contract.
	//
	//	4.	starknet_storage_leaf. Here the key suffix *is* the value so
	//			that could be returned without any lookup.
	return &GetValueResponse{Value: []byte(db[string(request.GetKey())])}, nil
}

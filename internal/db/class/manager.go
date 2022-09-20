package class

import (
	"math/big"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"
	"google.golang.org/protobuf/proto"
)

type ClassManager interface {
	GetClass(classHash *felt.Felt) (*types.ContractClass, error)
	PutClass(classHash *felt.Felt, class *types.ContractClass) error
	Close()
}

type classManager struct {
	database db.Database
}

func NewClassManager(database db.Database) ClassManager {
	return &classManager{
		database: database,
	}
}

func (c *classManager) GetClass(classHash *felt.Felt) (*types.ContractClass, error) {
	data, err := c.database.Get(classHash.ByteSlice())
	if err != nil {
		return nil, err
	}
	return unmarsalClass(data)
}

func (c *classManager) PutClass(classHash *felt.Felt, class *types.ContractClass) error {
	data, err := marshalClass(class)
	if err != nil {
		return err
	}
	return c.database.Put(classHash.ByteSlice(), data)
}

func (c *classManager) Close() {
	c.database.Close()
}

func marshalClass(class *types.ContractClass) ([]byte, error) {
	classPb := ContractClass{
		Program:     class.Program,
		Abi:         class.Abi,
		EntryPoints: entryPointsToPb(class.EntryPointsByType),
	}
	return proto.Marshal(&classPb)
}

func unmarsalClass(data []byte) (*types.ContractClass, error) {
	var classPb ContractClass
	if err := proto.Unmarshal(data, &classPb); err != nil {
		return nil, err
	}
	return &types.ContractClass{
		Program:           classPb.Program,
		Abi:               classPb.Abi,
		EntryPointsByType: entryPointsFromPb(classPb.EntryPoints),
	}, nil
}

func entryPointsToPb(entryPoints types.EntryPoints) *EntryPoints {
	constructors := make([]*EntryPoint, 0, len(entryPoints.Constructor))
	for _, item := range entryPoints.Constructor {
		constructors = append(constructors, entryPointToPb(item))
	}
	externals := make([]*EntryPoint, 0, len(entryPoints.External))
	for _, item := range entryPoints.External {
		externals = append(externals, entryPointToPb(item))
	}
	l1Handlres := make([]*EntryPoint, 0, len(entryPoints.L1Handler))
	for _, item := range entryPoints.L1Handler {
		l1Handlres = append(l1Handlres, entryPointToPb(item))
	}
	return &EntryPoints{
		Constructor: constructors,
		External:    externals,
		L1Handler:   l1Handlres,
	}
}

func entryPointsFromPb(entryPointsPb *EntryPoints) types.EntryPoints {
	constructors := make([]*types.ContractEntryPoint, 0, len(entryPointsPb.Constructor))
	for _, item := range entryPointsPb.Constructor {
		constructors = append(constructors, entryPointFromPb(item))
	}
	externals := make([]*types.ContractEntryPoint, 0, len(entryPointsPb.External))
	for _, item := range entryPointsPb.External {
		externals = append(externals, entryPointFromPb(item))
	}
	l1Handlers := make([]*types.ContractEntryPoint, 0, len(entryPointsPb.L1Handler))
	for _, item := range entryPointsPb.L1Handler {
		l1Handlers = append(l1Handlers, entryPointFromPb(item))
	}
	return types.EntryPoints{
		Constructor: constructors,
		External:    externals,
		L1Handler:   l1Handlers,
	}
}

func entryPointToPb(entryPoint *types.ContractEntryPoint) *EntryPoint {
	return &EntryPoint{
		Offset:   entryPoint.Offset.Bytes(),
		Selector: entryPoint.Selector.ByteSlice(),
	}
}

func entryPointFromPb(entryPointPb *EntryPoint) *types.ContractEntryPoint {
	return &types.ContractEntryPoint{
		Offset:   new(big.Int).SetBytes(entryPointPb.Offset),
		Selector: new(felt.Felt).SetBytes(entryPointPb.Selector),
	}
}

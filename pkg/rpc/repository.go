package rpc

type (
	// A MethodRepository has JSON-RPC method functions.
	MethodRepository struct {
		StructRpc interface{}
	}
)

func NewMethodRepositoryWithMethods(rpc interface{}) *MethodRepository {
	return &MethodRepository{
		StructRpc: rpc,
	}
}

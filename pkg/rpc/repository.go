package rpc

type (
	// A MethodRepository has JSON-RPC method functions.
	MethodRepository struct {
		MethodsToCall interface{}
	}
)

func NewMethodRepositoryWithMethods(methodCaller interface{}) *MethodRepository {
	return &MethodRepository{
		MethodsToCall: methodCaller,
	}
}

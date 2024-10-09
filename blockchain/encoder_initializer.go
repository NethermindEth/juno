package blockchain

import (
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/encoder"
)

var once sync.Once

func RegisterCoreTypesToEncoder() {
	once.Do(func() {
		types := []reflect.Type{
			reflect.TypeOf(core.DeclareTransaction{}),
			reflect.TypeOf(core.DeployTransaction{}),
			reflect.TypeOf(core.InvokeTransaction{}),
			reflect.TypeOf(core.L1HandlerTransaction{}),
			reflect.TypeOf(core.DeployAccountTransaction{}),
			reflect.TypeOf(core.Cairo0Class{}),
			reflect.TypeOf(core.Cairo1Class{}),
			reflect.TypeOf(core.StateContract{}),
		}

		for _, t := range types {
			err := encoder.RegisterType(t)
			if err != nil {
				panic(err)
			}
		}
	})
}

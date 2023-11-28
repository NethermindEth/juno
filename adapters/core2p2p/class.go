package core2p2p

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
)

func AdaptClass(class core.Class, compiledHash *felt.Felt) *spec.Class {
	if class == nil {
		return nil
	}

	switch v := class.(type) {
	case *core.Cairo0Class:
		return &spec.Class{
			CompiledHash: AdaptHash(compiledHash),
			Definition:   []byte(v.Program),
		}
	case *core.Cairo1Class:
		return &spec.Class{
			CompiledHash: AdaptHash(compiledHash),
			Definition:   v.Compiled,
		}
	default:
		panic(fmt.Errorf("unsupported cairo class %T (version=%d)", v, class.Version()))
	}
}

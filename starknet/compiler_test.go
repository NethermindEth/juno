package starknet_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/feeder"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompile(t *testing.T) {
	t.Run("zero sierra", func(t *testing.T) {
		_, err := starknet.Compile(&starknet.SierraDefinition{})
		require.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		cl := feeder.NewTestClient(t, utils.Integration)
		classHash := utils.HexToFelt(t, "0xc6c634d10e2cc7b1db6b4403b477f05e39cb4900fd5ea0156d1721dbb6c59b")

		classDef, err := cl.ClassDefinition(context.Background(), classHash)
		require.NoError(t, err)
		compiledDef, err := cl.CompiledClassDefinition(context.Background(), classHash)
		require.NoError(t, err)

		expectedCompiled, err := sn2core.AdaptCompiledClass(compiledDef)
		require.NoError(t, err)

		res, err := starknet.Compile(classDef.V1)
		require.NoError(t, err)

		gotCompiled, err := sn2core.AdaptCompiledClass(res)
		require.NoError(t, err)
		assert.Equal(t, expectedCompiled.Hash(), gotCompiled.Hash())
	})
}

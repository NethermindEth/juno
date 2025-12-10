package compiler_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompile(t *testing.T) {
	t.Run("zero sierra", func(t *testing.T) {
		_, err := compiler.Compile(&starknet.SierraClass{})
		require.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		cl := feeder.NewTestClient(t, &utils.Integration)
		classHash := felt.NewUnsafeFromString[felt.Felt]("0xc6c634d10e2cc7b1db6b4403b477f05e39cb4900fd5ea0156d1721dbb6c59b")

		classDef, err := cl.ClassDefinition(t.Context(), classHash)
		require.NoError(t, err)
		compiledDef, err := cl.CasmClassDefinition(t.Context(), classHash)
		require.NoError(t, err)

		expectedCompiled, err := sn2core.AdaptCasmClass(compiledDef)
		require.NoError(t, err)

		res, err := compiler.Compile(classDef.Sierra)
		require.NoError(t, err)

		gotCompiled, err := sn2core.AdaptCasmClass(res)
		require.NoError(t, err)
		assert.Equal(
			t,
			expectedCompiled.Hash(core.HashVersionV1),
			gotCompiled.Hash(core.HashVersionV1),
		)
	})

	t.Run("declare cairo2 class", func(t *testing.T) {
		// tests https://github.com/NethermindEth/juno/issues/1748
		definition := loadTestData[starknet.SierraClass](t, "declare_cairo2_definition.json")

		_, err := compiler.Compile(&definition)
		require.NoError(t, err)
	})
}

// loadTestData loads json file located relative to a test package and unmarshal it to provided type
func loadTestData[T any](t *testing.T, filename string) T {
	t.Helper()

	file := fmt.Sprintf("../testdata/%s", filename)
	buff, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", file, err)
	}

	var v T
	err = json.Unmarshal(buff, &v)
	if err != nil {
		t.Fatalf("Failed to unmarshal json: %v", err)
	}

	// todo check for zero value
	return v
}

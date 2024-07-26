package starknet_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/feeder"
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
		cl := feeder.NewTestClient(t, &utils.Mainnet)
		classHash := utils.HexToFelt(t, "0x1338d85d3e579f6944ba06c005238d145920afeb32f94e3a1e234d21e1e9292")

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

	t.Run("declare cairo2 class", func(t *testing.T) {
		// tests https://github.com/NethermindEth/juno/issues/1748
		definition := loadTestData[starknet.SierraDefinition](t, "declare_cairo2_definition.json")

		_, err := starknet.Compile(&definition)
		require.NoError(t, err)
	})
}

// loadTestData loads json file located relative to a test package and unmarshal it to provided type
func loadTestData[T any](t *testing.T, filename string) T {
	t.Helper()

	file := fmt.Sprintf("./testdata/%s", filename)
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

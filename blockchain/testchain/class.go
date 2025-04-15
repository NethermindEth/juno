package testchain

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/hash"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/stretchr/testify/require"
)

type classDefinition struct {
	t         *testing.T
	Hash      hash.ClassHash
	Instances []deployedContract
	Class     core.Cairo1Class
	Sierra    starknet.SierraDefinition
}

func NewClass(t *testing.T, path string) classDefinition {
	t.Helper()

	sierra, classDef := classFromFile(t, path)
	classHash, err := classDef.Hash()
	require.NoError(t, err)

	return classDefinition{
		t:      t,
		Hash:   hash.ClassHash(*classHash),
		Class:  classDef,
		Sierra: sierra,
	}
}

func (t *classDefinition) AddInstance(address *address.ContractAddress, balance *felt.Felt) {
	t.Instances = append(t.Instances, deployedContract{
		t:       t.t,
		address: *address,
		balance: *balance,
	})
}

func classFromFile(t *testing.T, path string) (
	starknet.SierraDefinition, core.Cairo1Class,
) {
	t.Helper()

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	intermediate := new(struct {
		Abi         json.RawMessage            `json:"abi"`
		EntryPoints starknet.SierraEntryPoints `json:"entry_points_by_type"`
		Program     []*felt.Felt               `json:"sierra_program"`
		Version     string                     `json:"contract_class_version"`
	})
	require.NoError(t, json.NewDecoder(file).Decode(intermediate))

	snSierra := starknet.SierraDefinition{
		Abi:         string(intermediate.Abi),
		EntryPoints: intermediate.EntryPoints,
		Program:     intermediate.Program,
		Version:     intermediate.Version,
	}

	snCasm, err := compiler.Compile(&snSierra)
	require.NoError(t, err)

	junoClass, err := sn2core.AdaptCairo1Class(&snSierra, snCasm)
	require.NoError(t, err)

	// TODO: I don't like this too much (the deference using *), but every method that's currently
	//       returning a reference type will eventually return a value type. Since this is for
	//       testing is not critical.
	return snSierra, *junoClass
}

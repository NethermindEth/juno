package core

import (
	"sync"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

func deprecatedCairoClassHash(class *DeprecatedCairoClass) (felt.Felt, error) {
	decompressedProgram, err := utils.Gzip64Decode(class.Program)
	if err != nil {
		return felt.Felt{}, err
	}

	program, err := unmarshalDeprecatedCairoProgram(decompressedProgram)
	if err != nil {
		return felt.Felt{}, err
	}

	var (
		externalEntryPointsHash    felt.Felt
		l1HandlerEntryPointsHash   felt.Felt
		constructorEntryPointsHash felt.Felt
		builtinsHash               felt.Felt
		dataHash                   felt.Felt
		hintedClassHash            felt.Felt
		hintedClassHashErr         error
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		dataHash = hashDeprecatedProgramData(program.Data)
	}()

	go func() {
		defer wg.Done()
		externalEntryPointsHash = hashDeprecatedEntryPoints(class.Externals)
		l1HandlerEntryPointsHash = hashDeprecatedEntryPoints(class.L1Handlers)
		constructorEntryPointsHash = hashDeprecatedEntryPoints(class.Constructors)
		builtinsHash = hashBuiltinNames(program.Builtins)
	}()

	hintedClassHash, hintedClassHashErr = computeHintedClassHash(class.Abi, program)

	wg.Wait()
	if hintedClassHashErr != nil {
		return felt.Felt{}, hintedClassHashErr
	}

	hash := crypto.PedersenArray(
		&felt.Zero,
		&externalEntryPointsHash,
		&l1HandlerEntryPointsHash,
		&constructorEntryPointsHash,
		&builtinsHash,
		&hintedClassHash,
		&dataHash,
	)
	return hash, nil
}

func hashDeprecatedEntryPoints(entryPoints []DeprecatedEntryPoint) felt.Felt {
	var digest crypto.PedersenDigest
	for i := range entryPoints {
		digest.Update(entryPoints[i].Selector, entryPoints[i].Offset)
	}
	return digest.Finish()
}

func hashBuiltinNames(builtinNames []string) felt.Felt {
	var digest crypto.PedersenDigest
	for i := range builtinNames {
		builtin := felt.FromBytes[felt.Felt]([]byte(builtinNames[i]))
		digest.Update(&builtin)
	}
	return digest.Finish()
}

func hashDeprecatedProgramData(data []felt.Felt) felt.Felt {
	var digest crypto.PedersenDigest
	for i := range data {
		digest.Update(&data[i])
	}
	return digest.Finish()
}

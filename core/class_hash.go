package core

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
)

func deprecatedCairoClassHash(class *DeprecatedCairoClass) (felt.Felt, error) {
	definition, err := makeDeprecatedVMClass(class)
	if err != nil {
		return felt.Felt{}, err
	}

	program, err := unmarshalDeprecatedCairoProgram(definition.Program)
	if err != nil {
		return felt.Felt{}, err
	}

	externalEntryPointsHash := hashDeprecatedEntryPoints(definition.EntryPoints.External)
	l1HandlerEntryPointsHash := hashDeprecatedEntryPoints(definition.EntryPoints.L1Handler)
	constructorEntryPointsHash := hashDeprecatedEntryPoints(definition.EntryPoints.Constructor)
	builtinsHash := hashBuiltinNames(program.Builtins)
	dataHash := hashDeprecatedProgramData(program.Data)
	hintedClassHash, err := computeHintedClassHash(definition.Abi, program)
	if err != nil {
		return felt.Felt{}, err
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

func hashDeprecatedEntryPoints(entryPoints []starknet.EntryPoint) felt.Felt {
	var digest crypto.PedersenDigest
	for i := range entryPoints {
		digest.Update(entryPoints[i].Selector, (*felt.Felt)(entryPoints[i].Offset))
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

func makeDeprecatedVMClass(class *DeprecatedCairoClass) (*starknet.DeprecatedCairoClass, error) {
	adaptEntryPoint := func(ep DeprecatedEntryPoint) starknet.EntryPoint {
		return starknet.EntryPoint{
			Selector: ep.Selector,
			Offset:   (*starknet.EntryPointOffset)(ep.Offset),
		}
	}

	constructors := utils.Map(utils.NonNilSlice(class.Constructors), adaptEntryPoint)
	external := utils.Map(utils.NonNilSlice(class.Externals), adaptEntryPoint)
	handlers := utils.Map(utils.NonNilSlice(class.L1Handlers), adaptEntryPoint)

	decompressedProgram, err := utils.Gzip64Decode(class.Program)
	if err != nil {
		return nil, err
	}

	return &starknet.DeprecatedCairoClass{
		Program: decompressedProgram,
		Abi:     class.Abi,
		EntryPoints: starknet.EntryPoints{
			Constructor: constructors,
			External:    external,
			L1Handler:   handlers,
		},
	}, nil
}

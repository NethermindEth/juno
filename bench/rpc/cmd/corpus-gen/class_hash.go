package main

import "fmt"

func classAtBlockSampler(input samplerInput[blockRangeFlags]) (any, error) {
	blockNumber := input.args.sampleBlockNumber(input.rng)
	classHash, err := sampleClassHash(input, blockNumber)
	if err != nil {
		return nil, err
	}
	return classAtBlockParams{BlockID: blockNumberID{blockNumber}, ClassHash: classHash}, nil
}

func sierraClassHashSampler(input samplerInput[blockRangeFlags]) (any, error) {
	classHash, err := sampleSierraClassHash(input, input.args.sampleBlockNumber(input.rng))
	if err != nil {
		return nil, err
	}
	return classHashParams{ClassHash: classHash}, nil
}

func sampleClassHash[T any](input samplerInput[T], blockNumber uint64) (string, error) {
	address, err := sampleContractAddress(input, blockNumber, storageDiffAddresses)
	if err != nil {
		return "", err
	}
	classHash, err := input.client.classHashAt(input.ctx, blockNumber, address)
	if err != nil {
		return "", fmt.Errorf("class hash of %s at block %d: %w", address, blockNumber, err)
	}
	return classHash, nil
}

// sampleSierraClassHash returns errResample for non-Sierra classes so the
// caller draws another block (Cairo 0 classes have no CASM).
func sampleSierraClassHash[T any](input samplerInput[T], blockNumber uint64) (string, error) {
	classHash, err := sampleClassHash(input, blockNumber)
	if err != nil {
		return "", err
	}
	class, err := input.client.classAt(input.ctx, blockNumber, classHash)
	if err != nil {
		return "", err
	}
	if len(class.SierraProgram) == 0 {
		return "", errResample
	}
	return classHash, nil
}

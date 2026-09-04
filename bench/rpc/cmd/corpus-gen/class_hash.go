package main

import "fmt"

func classAtBlockSampler(input samplerInput[blockIDArgs]) (classAtBlockParams, error) {
	contract, err := contractAddressSampler(storageDiffAddresses)(input)
	if err != nil {
		return classAtBlockParams{}, err
	}
	blockNumber := contract.BlockID.blockNumber()
	classHash, err := input.client.classHashAt(input.ctx, blockNumber, contract.ContractAddress)
	if err != nil {
		return classAtBlockParams{}, fmt.Errorf(
			"class hash of %s at block %d: %w", contract.ContractAddress, blockNumber, err,
		)
	}
	return classAtBlockParams{BlockID: contract.BlockID, ClassHash: classHash}, nil
}

func sierraClassHashSampler(input samplerInput[blockRangeFlags]) (classHashParams, error) {
	params, err := classAtBlockSampler(
		input.rebindArgs(&blockIDArgs{blockRangeFlags: *input.args, BlockIDKind: blockIDNumber}),
	)
	if err != nil {
		return classHashParams{}, err
	}
	isSierra, err := input.cache.isSierra(
		input.ctx, input.client, params.BlockID.blockNumber(), params.ClassHash,
	)
	if err != nil {
		return classHashParams{}, err
	}
	if !isSierra {
		// Cairo 0 classes have no CASM; resample another block.
		return classHashParams{}, errResample
	}
	return classHashParams{ClassHash: params.ClassHash}, nil
}

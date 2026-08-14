package main

import (
	"math/big"
	"slices"
	"strings"
)

func contractAddressSampler(listAddresses func(stateDiff) []string) sampler[blockRangeFlags] {
	return func(input samplerInput[blockRangeFlags]) (any, error) {
		blockNumber := input.args.sampleBlockNumber(input.rng)
		address, err := sampleContractAddress(input, blockNumber, listAddresses)
		if err != nil {
			return nil, err
		}
		return contractAtBlockParams{
			BlockID:         blockNumberID{blockNumber},
			ContractAddress: address,
		}, nil
	}
}

func sampleContractAddress[T any](
	input samplerInput[T],
	blockNumber uint64,
	listAddresses func(stateDiff) []string,
) (string, error) {
	update, err := input.client.stateUpdateAt(input.ctx, blockNumber)
	if err != nil {
		return "", err
	}
	return pickRandom(input.rng, listAddresses(update.StateDiff))
}

func storageDiffAddresses(diff stateDiff) []string {
	addresses := make([]string, 0, len(diff.StorageDiffs))
	for _, d := range diff.StorageDiffs {
		if !isSystemContract(d.Address) {
			addresses = append(addresses, d.Address)
		}
	}
	// Nodes may serve state diff arrays in arbitrary per-call order; sort so
	// pickRandom stays reproducible for a given seed.
	slices.Sort(addresses)
	return addresses
}

func nonceAddresses(diff stateDiff) []string {
	addresses := make([]string, 0, len(diff.Nonces))
	for _, n := range diff.Nonces {
		addresses = append(addresses, n.ContractAddress)
	}
	slices.Sort(addresses)
	return addresses
}

// Addresses below 0x10 are reserved system contracts (e.g. 0x1 stores block
// hashes); they take storage writes but have no deployed class.
const systemContractLimit = 0x10

func isSystemContract(address string) bool {
	value, ok := new(big.Int).SetString(strings.TrimPrefix(address, "0x"), 16)
	return !ok || value.Cmp(big.NewInt(systemContractLimit)) < 0
}

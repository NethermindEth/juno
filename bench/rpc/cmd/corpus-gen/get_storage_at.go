package main

import (
	"cmp"
	"slices"
)

func storageAtSampler(input samplerInput[blockRangeFlags]) (storageAtParams, error) {
	blockNumber := input.args.sampleBlockNumber(input.rng)
	update, err := input.client.stateUpdateAt(input.ctx, blockNumber)
	if err != nil {
		return storageAtParams{}, err
	}
	entry, err := pickRandom(input.rng, storageEntries(update.StateDiff))
	if err != nil {
		return storageAtParams{}, err
	}
	return storageAtParams{
		ContractAddress: entry.Address,
		Key:             entry.Key,
		BlockID:         blockNumberID{blockNumber},
	}, nil
}

type storageEntry struct {
	Address string
	Key     string
}

func storageEntries(diff stateDiff) []storageEntry {
	var entries []storageEntry
	for _, d := range diff.StorageDiffs {
		if isSystemContract(d.Address) {
			continue
		}
		for _, e := range d.StorageEntries {
			entries = append(entries, storageEntry{Address: d.Address, Key: e.Key})
		}
	}
	// Nodes may serve state diff arrays in arbitrary per-call order; sort so
	// pickRandom stays reproducible for a given seed.
	slices.SortFunc(entries, func(a, b storageEntry) int {
		return cmp.Or(cmp.Compare(a.Address, b.Address), cmp.Compare(a.Key, b.Key))
	})
	return entries
}

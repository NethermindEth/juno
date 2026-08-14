package main

import (
	"cmp"
	"slices"
)

func storageAtSampler(input samplerInput[blockRangeFlags]) (any, error) {
	blockNumber := input.args.sampleBlockNumber(input.rng)
	entry, err := sampleStorageEntry(input, blockNumber)
	if err != nil {
		return nil, err
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

func sampleStorageEntry[T any](input samplerInput[T], blockNumber uint64) (storageEntry, error) {
	update, err := input.client.stateUpdateAt(input.ctx, blockNumber)
	if err != nil {
		return storageEntry{}, err
	}
	return pickRandom(input.rng, storageEntries(update.StateDiff))
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

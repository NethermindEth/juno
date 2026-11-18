package main

import (
	"cmp"
	"slices"

	"github.com/spf13/cobra"
)

type storageAtArgs struct {
	blockIDArgs
	ResponseFlags storageAtFlags `json:"responseFlags"`
}

func (a *storageAtArgs) bind(cmd *cobra.Command, client *rpcClient) {
	a.ResponseFlags.bind(cmd, client)
	a.blockIDArgs.bind(cmd, client)
}

func storageAtSampler(input samplerInput[storageAtArgs]) (storageAtParams, error) {
	blockNumber := input.args.sampleBlockNumber(input.rng)
	update, err := input.client.stateUpdateAt(input.ctx, blockNumber)
	if err != nil {
		return storageAtParams{}, err
	}
	entry, err := pickRandom(input.rng, storageEntries(update.StateDiff))
	if err != nil {
		return storageAtParams{}, err
	}
	id, err := resolveBlockID(input.ctx, input.client, input.args.BlockIDKind, blockNumber)
	if err != nil {
		return storageAtParams{}, err
	}
	return storageAtParams{
		ContractAddress: entry.Address,
		Key:             entry.Key,
		BlockID:         id,
		ResponseFlags:   input.args.ResponseFlags,
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

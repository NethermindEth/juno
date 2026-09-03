package rpcv10

import "github.com/NethermindEth/juno/core/felt"

func emptyInitialReads() *InitialReads {
	return &InitialReads{
		Storage:           []StorageEntry{},
		Nonces:            []NonceEntry{},
		ClassHashes:       []ClassHashEntry{},
		DeclaredContracts: []DeclaredContractEntry{},
	}
}

// mergeInitialReads requires non-nil incoming reads and retains the earliest value for each key.
func mergeInitialReads(existing, incoming *InitialReads) *InitialReads {
	if existing == nil {
		return incoming
	}

	type storageKey struct {
		address felt.Address
		key     felt.Felt
	}
	storageSeen := make(map[storageKey]struct{}, len(existing.Storage))
	for _, read := range existing.Storage {
		storageSeen[storageKey{address: read.ContractAddress, key: read.Key}] = struct{}{}
	}
	for _, read := range incoming.Storage {
		key := storageKey{address: read.ContractAddress, key: read.Key}
		if _, found := storageSeen[key]; !found {
			existing.Storage = append(existing.Storage, read)
			storageSeen[key] = struct{}{}
		}
	}

	nonceSeen := make(map[felt.Address]struct{}, len(existing.Nonces))
	for _, read := range existing.Nonces {
		nonceSeen[read.ContractAddress] = struct{}{}
	}
	for _, read := range incoming.Nonces {
		if _, found := nonceSeen[read.ContractAddress]; !found {
			existing.Nonces = append(existing.Nonces, read)
			nonceSeen[read.ContractAddress] = struct{}{}
		}
	}

	classHashSeen := make(map[felt.Address]struct{}, len(existing.ClassHashes))
	for _, read := range existing.ClassHashes {
		classHashSeen[read.ContractAddress] = struct{}{}
	}
	for _, read := range incoming.ClassHashes {
		if _, found := classHashSeen[read.ContractAddress]; !found {
			existing.ClassHashes = append(existing.ClassHashes, read)
			classHashSeen[read.ContractAddress] = struct{}{}
		}
	}

	declaredSeen := make(map[felt.ClassHash]struct{}, len(existing.DeclaredContracts))
	for _, read := range existing.DeclaredContracts {
		declaredSeen[read.ClassHash] = struct{}{}
	}
	for _, read := range incoming.DeclaredContracts {
		if _, found := declaredSeen[read.ClassHash]; !found {
			existing.DeclaredContracts = append(existing.DeclaredContracts, read)
			declaredSeen[read.ClassHash] = struct{}{}
		}
	}
	return existing
}

package core

import (
	"fmt"
	"slices"
	"sort"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

type StateUpdate struct {
	BlockHash *felt.Felt
	NewRoot   *felt.Felt
	OldRoot   *felt.Felt
	StateDiff *StateDiff
}

type StateDiff struct {
	StorageDiffs      map[felt.Felt]map[felt.Felt]*felt.Felt // addr -> {key -> value, ...}
	Nonces            map[felt.Felt]*felt.Felt               // addr -> nonce
	DeployedContracts map[felt.Felt]*felt.Felt               // addr -> class hash
	DeclaredV0Classes []*felt.Felt                           // class hashes
	DeclaredV1Classes map[felt.Felt]*felt.Felt               // class hash -> compiled class hash
	ReplacedClasses   map[felt.Felt]*felt.Felt               // addr -> class hash
}

func EmptyStateDiff() *StateDiff {
	return &StateDiff{
		StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt),
		Nonces:            make(map[felt.Felt]*felt.Felt),
		DeployedContracts: make(map[felt.Felt]*felt.Felt),
		DeclaredV0Classes: make([]*felt.Felt, 0),
		DeclaredV1Classes: make(map[felt.Felt]*felt.Felt),
		ReplacedClasses:   make(map[felt.Felt]*felt.Felt),
	}
}

func (d *StateDiff) Length() uint64 {
	var length int

	for _, storageDiff := range d.StorageDiffs {
		length += len(storageDiff)
	}
	length += len(d.Nonces)
	length += len(d.DeployedContracts)
	length += len(d.DeclaredV0Classes)
	length += len(d.DeclaredV1Classes)
	length += len(d.ReplacedClasses)

	return uint64(length)
}

func (d *StateDiff) Hash() *felt.Felt {
	var digest crypto.PoseidonDigest

	digest.Update(new(felt.Felt).SetBytes([]byte("STARKNET_STATE_DIFF0")))

	// updated_contracts = deployedContracts + replacedClasses
	// Digest: [number_of_updated_contracts, address_0, class_hash_0, address_1, class_hash_1, ...].
	numOfUpdatedContracts := uint64(len(d.DeployedContracts) + len(d.ReplacedClasses))
	digest.Update(new(felt.Felt).SetUint64(numOfUpdatedContracts))

	// The sequencer guarantees that a contract cannot be:
	// - deployed twice,
	// - deployed and have its class replaced in the same state diff, or
	// - have its class replaced multiple times in the same state diff.
	updatedContracts := make(map[felt.Felt]*felt.Felt)
	for k, v := range d.DeployedContracts {
		updatedContracts[k] = v
	}
	for k, v := range d.ReplacedClasses {
		updatedContracts[k] = v
	}

	sortedUpdatedContractsHashes := sortedFeltKeys(updatedContracts)
	for _, hash := range sortedUpdatedContractsHashes {
		digest.Update(&hash, updatedContracts[hash])
	}

	// declared classes
	// Digest: [number_of_declared_classes, class_hash_0, compiled_class_hash_0, class_hash_1, compiled_class_hash_1,
	// ...].
	numOfDeclaredClasses := uint64(len(d.DeclaredV1Classes))
	digest.Update(new(felt.Felt).SetUint64(numOfDeclaredClasses))

	sortedDeclaredV1ClassHashes := sortedFeltKeys(d.DeclaredV1Classes)
	for _, classHash := range sortedDeclaredV1ClassHashes {
		digest.Update(&classHash, d.DeclaredV1Classes[classHash])
	}

	// deprecated_declared_classes
	// Digest: [number_of_old_declared_classes, class_hash_0, class_hash_1, ...].
	numOfDeclaredV0Classes := uint64(len(d.DeclaredV0Classes))
	digest.Update(new(felt.Felt).SetUint64(numOfDeclaredV0Classes))

	// Todo: consider make a copy of d.DeclaredV0Classes?
	slices.SortFunc(d.DeclaredV0Classes, func(a, b *felt.Felt) int { return a.Cmp(b) })
	digest.Update(d.DeclaredV0Classes...)

	// Placeholder values
	digest.Update(new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(0))

	// storage_diffs
	// Digest: [
	//	number_of_updated_contracts,
	//  contract_address_0, number_of_updates_in_contract_0, key_0, value0, key1, value1, ...,
	//  contract_address_1, number_of_updates_in_contract_1, key_0, value0, key1, value1, ...,
	// ]
	numOfStorageDiffs := uint64(len(d.StorageDiffs))
	digest.Update(new(felt.Felt).SetUint64(numOfStorageDiffs))

	sortedStorageDiffAddrs := sortedFeltKeys(d.StorageDiffs)
	for _, addr := range sortedStorageDiffAddrs {
		digest.Update(&addr)

		sortedDiffKeys := sortedFeltKeys(d.StorageDiffs[addr])
		digest.Update(new(felt.Felt).SetUint64(uint64(len(sortedDiffKeys))))

		for _, k := range sortedDiffKeys {
			digest.Update(&k, d.StorageDiffs[addr][k])
		}
	}

	// nonces
	// Digest: [number_of_updated_contracts nonces, contract_address_0, nonce_0, contract_address_1, nonce_1, ...]
	numOfNonces := uint64(len(d.Nonces))
	digest.Update(new(felt.Felt).SetUint64(numOfNonces))

	sortedNoncesAddrs := sortedFeltKeys(d.Nonces)
	for _, addr := range sortedNoncesAddrs {
		digest.Update(&addr, d.Nonces[addr])
	}

	/*Poseidon(
	    "STARKNET_STATE_DIFF0", deployed_contracts_and_replaced_classes, declared_classes, deprecated_declared_classes,
	    1, 0, storage_diffs, nonces
	)*/
	return digest.Finish()
}

func (d *StateDiff) Commitment() *felt.Felt {
	version := felt.Zero
	var tmpFelt felt.Felt

	/*
		hash_of_deployed_contracts=hash([number_of_deployed_contracts, address_1, class_hash_1,
			address_2, class_hash_2, ...])
	*/
	var hashOfDeployedContracts crypto.PoseidonDigest
	deployedReplacedAddresses := make([]felt.Felt, 0, len(d.DeployedContracts)+len(d.ReplacedClasses))
	for addr := range d.DeployedContracts {
		deployedReplacedAddresses = append(deployedReplacedAddresses, addr)
	}
	for addr := range d.ReplacedClasses {
		deployedReplacedAddresses = append(deployedReplacedAddresses, addr)
	}
	hashOfDeployedContracts.Update(tmpFelt.SetUint64(uint64(len(deployedReplacedAddresses))))
	sort.Slice(deployedReplacedAddresses, func(i, j int) bool {
		switch deployedReplacedAddresses[i].Cmp(&deployedReplacedAddresses[j]) {
		case -1:
			return true
		case 1:
			return false
		default:
			// The sequencer guarantees that a contract cannot be:
			// - deployed twice,
			// - deployed and have its class replaced in the same state diff, or
			// - have its class replaced multiple times in the same state diff.
			panic(fmt.Sprintf("address appears twice in deployed and replaced addresses: %s", &deployedReplacedAddresses[i]))
		}
	})
	for idx := range deployedReplacedAddresses {
		addr := deployedReplacedAddresses[idx]
		classHash, ok := d.DeployedContracts[addr]
		if !ok {
			classHash = d.ReplacedClasses[addr]
		}
		hashOfDeployedContracts.Update(&addr, classHash)
	}

	/*
		hash_of_declared_classes = hash([number_of_declared_classes, class_hash_1, compiled_class_hash_1,
			class_hash_2, compiled_class_hash_2, ...])
	*/
	var hashOfDeclaredClasses crypto.PoseidonDigest
	hashOfDeclaredClasses.Update(tmpFelt.SetUint64(uint64(len(d.DeclaredV1Classes))))
	declaredV1ClassHashes := sortedFeltKeys(d.DeclaredV1Classes)
	for idx := range declaredV1ClassHashes {
		classHash := declaredV1ClassHashes[idx]
		hashOfDeclaredClasses.Update(&classHash, d.DeclaredV1Classes[classHash])
	}

	/*
		hash_of_old_declared_classes = hash([number_of_old_declared_classes, class_hash_1, class_hash_2, ...])
	*/
	var hashOfOldDeclaredClasses crypto.PoseidonDigest
	hashOfOldDeclaredClasses.Update(tmpFelt.SetUint64(uint64(len(d.DeclaredV0Classes))))
	sort.Slice(d.DeclaredV0Classes, func(i, j int) bool {
		return d.DeclaredV0Classes[i].Cmp(d.DeclaredV0Classes[j]) == -1
	})
	hashOfOldDeclaredClasses.Update(d.DeclaredV0Classes...)

	/*
		flattened_storage_diffs = [number_of_updated_contracts, contract_address_1, number_of_updates_in_contract,
			key_1, value_1, key_2, value_2, ..., contract_address_2, number_of_updates_in_contract, ...]
		flattened_nonces = [number_of_updated_contracts, address_1, nonce_1, address_2, nonce_2, ...]
		hash_of_storage_domain_state_diff = hash([*flattened_storage_diffs, *flattened_nonces])
	*/
	daModeL1 := 0
	hashOfStorageDomains := make([]crypto.PoseidonDigest, 1)

	sortedStorageDiffAddrs := sortedFeltKeys(d.StorageDiffs)
	hashOfStorageDomains[daModeL1].Update(tmpFelt.SetUint64(uint64(len(sortedStorageDiffAddrs))))
	for idx, addr := range sortedStorageDiffAddrs {
		hashOfStorageDomains[daModeL1].Update(&sortedStorageDiffAddrs[idx])
		diffKeys := sortedFeltKeys(d.StorageDiffs[sortedStorageDiffAddrs[idx]])

		hashOfStorageDomains[daModeL1].Update(tmpFelt.SetUint64(uint64(len(diffKeys))))
		for idx := range diffKeys {
			key := diffKeys[idx]
			hashOfStorageDomains[daModeL1].Update(&key, d.StorageDiffs[addr][key])
		}
	}

	sortedNonceKeys := sortedFeltKeys(d.Nonces)
	hashOfStorageDomains[daModeL1].Update(tmpFelt.SetUint64(uint64(len(sortedNonceKeys))))
	for idx := range sortedNonceKeys {
		hashOfStorageDomains[daModeL1].Update(&sortedNonceKeys[idx], d.Nonces[sortedNonceKeys[idx]])
	}

	/*
		flattened_total_state_diff = hash([state_diff_version,
			hash_of_deployed_contracts, hash_of_declared_classes,
			hash_of_old_declared_classes, number_of_DA_modes,
			DA_mode_0, hash_of_storage_domain_state_diff_0, DA_mode_1, hash_of_storage_domain_state_diff_1, …])
	*/
	var commitmentDigest crypto.PoseidonDigest
	commitmentDigest.Update(&version, hashOfDeployedContracts.Finish(), hashOfDeclaredClasses.Finish(), hashOfOldDeclaredClasses.Finish())
	commitmentDigest.Update(tmpFelt.SetUint64(uint64(len(hashOfStorageDomains))))
	for idx := range hashOfStorageDomains {
		commitmentDigest.Update(tmpFelt.SetUint64(uint64(idx)), hashOfStorageDomains[idx].Finish())
	}
	return commitmentDigest.Finish()
}

func sortedFeltKeys[V any](m map[felt.Felt]V) []felt.Felt {
	keys := make([]felt.Felt, 0, len(m))
	for addr := range m {
		keys = append(keys, addr)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Cmp(&keys[j]) == -1
	})

	return keys
}

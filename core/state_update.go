package core

import (
	"maps"
	"slices"

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
	digest := new(crypto.PoseidonDigest)

	digest.Update(new(felt.Felt).SetBytes([]byte("STARKNET_STATE_DIFF0")))

	// updated_contracts = deployedContracts + replacedClasses
	// Digest: [number_of_updated_contracts, address_0, class_hash_0, address_1, class_hash_1, ...].
	updatedContractsDigest(d.DeployedContracts, d.ReplacedClasses, digest)

	// declared classes
	// Digest: [number_of_declared_classes, class_hash_0, compiled_class_hash_0, class_hash_1, compiled_class_hash_1,
	// ...].
	declaredClassesDigest(d.DeclaredV1Classes, digest)

	// deprecated_declared_classes
	// Digest: [number_of_old_declared_classes, class_hash_0, class_hash_1, ...].
	deprecatedDeclaredClassesDigest(d.DeclaredV0Classes, digest)

	// Placeholder values
	digest.Update(new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(0))

	// storage_diffs
	// Digest: [
	//	number_of_updated_contracts,
	//  contract_address_0, number_of_updates_in_contract_0, key_0, value0, key1, value1, ...,
	//  contract_address_1, number_of_updates_in_contract_1, key_0, value0, key1, value1, ...,
	// ]
	storageDiffDigest(d.StorageDiffs, digest)

	// nonces
	// Digest: [number_of_updated_contracts nonces, contract_address_0, nonce_0, contract_address_1, nonce_1, ...]
	noncesDigest(d.Nonces, digest)

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
	hashOfDeployedContracts := new(crypto.PoseidonDigest)
	updatedContractsDigest(d.DeployedContracts, d.ReplacedClasses, hashOfDeployedContracts)

	/*
		hash_of_declared_classes = hash([number_of_declared_classes, class_hash_1, compiled_class_hash_1,
			class_hash_2, compiled_class_hash_2, ...])
	*/
	hashOfDeclaredClasses := new(crypto.PoseidonDigest)
	declaredClassesDigest(d.DeclaredV1Classes, hashOfDeclaredClasses)

	/*
		hash_of_old_declared_classes = hash([number_of_old_declared_classes, class_hash_1, class_hash_2, ...])
	*/
	hashOfOldDeclaredClasses := new(crypto.PoseidonDigest)
	deprecatedDeclaredClassesDigest(d.DeclaredV0Classes, hashOfOldDeclaredClasses)

	/*
		flattened_storage_diffs = [number_of_updated_contracts, contract_address_1, number_of_updates_in_contract,
			key_1, value_1, key_2, value_2, ..., contract_address_2, number_of_updates_in_contract, ...]
		flattened_nonces = [number_of_updated_contracts, address_1, nonce_1, address_2, nonce_2, ...]
		hash_of_storage_domain_state_diff = hash([*flattened_storage_diffs, *flattened_nonces])
	*/
	daModeL1 := 0
	hashOfStorageDomains := make([]*crypto.PoseidonDigest, 1)
	hashOfStorageDomains[daModeL1] = new(crypto.PoseidonDigest)

	storageDiffDigest(d.StorageDiffs, hashOfStorageDomains[daModeL1])
	noncesDigest(d.Nonces, hashOfStorageDomains[daModeL1])

	/*
		flattened_total_state_diff = hash([state_diff_version,
			hash_of_deployed_contracts, hash_of_declared_classes,
			hash_of_old_declared_classes, number_of_DA_modes,
			DA_mode_0, hash_of_storage_domain_state_diff_0, DA_mode_1, hash_of_storage_domain_state_diff_1, …])
	*/
	commitmentDigest := new(crypto.PoseidonDigest)
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
	slices.SortFunc(keys, func(a, b felt.Felt) int { return a.Cmp(&b) })
	return keys
}

func updatedContractsDigest(deployedContracts, replacedClasses map[felt.Felt]*felt.Felt, digest *crypto.PoseidonDigest) {
	numOfUpdatedContracts := uint64(len(deployedContracts) + len(replacedClasses))
	digest.Update(new(felt.Felt).SetUint64(numOfUpdatedContracts))

	// The sequencer guarantees that a contract cannot be:
	// - deployed twice,
	// - deployed and have its class replaced in the same state diff, or
	// - have its class replaced multiple times in the same state diff.
	updatedContracts := make(map[felt.Felt]*felt.Felt)
	maps.Copy(updatedContracts, deployedContracts)
	maps.Copy(updatedContracts, replacedClasses)

	sortedUpdatedContractsHashes := sortedFeltKeys(updatedContracts)
	for _, hash := range sortedUpdatedContractsHashes {
		digest.Update(&hash, updatedContracts[hash])
	}
}

func declaredClassesDigest(declaredV1Classes map[felt.Felt]*felt.Felt, digest *crypto.PoseidonDigest) {
	numOfDeclaredClasses := uint64(len(declaredV1Classes))
	digest.Update(new(felt.Felt).SetUint64(numOfDeclaredClasses))

	sortedDeclaredV1ClassHashes := sortedFeltKeys(declaredV1Classes)
	for _, classHash := range sortedDeclaredV1ClassHashes {
		digest.Update(&classHash, declaredV1Classes[classHash])
	}
}

func deprecatedDeclaredClassesDigest(declaredV0Classes []*felt.Felt, digest *crypto.PoseidonDigest) {
	numOfDeclaredV0Classes := uint64(len(declaredV0Classes))
	digest.Update(new(felt.Felt).SetUint64(numOfDeclaredV0Classes))

	slices.SortFunc(declaredV0Classes, func(a, b *felt.Felt) int { return a.Cmp(b) })
	digest.Update(declaredV0Classes...)
}

func storageDiffDigest(storageDiffs map[felt.Felt]map[felt.Felt]*felt.Felt, digest *crypto.PoseidonDigest) {
	numOfStorageDiffs := uint64(len(storageDiffs))
	digest.Update(new(felt.Felt).SetUint64(numOfStorageDiffs))

	sortedStorageDiffAddrs := sortedFeltKeys(storageDiffs)
	for _, addr := range sortedStorageDiffAddrs {
		digest.Update(&addr)

		sortedDiffKeys := sortedFeltKeys(storageDiffs[addr])
		digest.Update(new(felt.Felt).SetUint64(uint64(len(sortedDiffKeys))))

		for _, k := range sortedDiffKeys {
			digest.Update(&k, storageDiffs[addr][k])
		}
	}
}

func noncesDigest(nonces map[felt.Felt]*felt.Felt, digest *crypto.PoseidonDigest) {
	numOfNonces := uint64(len(nonces))
	digest.Update(new(felt.Felt).SetUint64(numOfNonces))

	sortedNoncesAddrs := sortedFeltKeys(nonces)
	for _, addr := range sortedNoncesAddrs {
		digest.Update(&addr, nonces[addr])
	}
}

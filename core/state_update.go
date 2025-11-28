package core

import (
	"maps"
	"slices"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

type StateUpdate struct {
	BlockHash *felt.Felt `cbor:"1,keyasint,omitempty"`
	NewRoot   *felt.Felt `cbor:"2,keyasint,omitempty"`
	OldRoot   *felt.Felt `cbor:"3,keyasint,omitempty"`
	StateDiff *StateDiff `cbor:"4,keyasint,omitempty"`
}

type StateDiff struct {
	// todo(rdr): replace felt.Felt for the right types (felt.Address to felt.What? to felt.What?)
	//            felt.What? means I'm not sure which type, but if it doesn't exist, create it.
	StorageDiffs map[felt.Felt]map[felt.Felt]*felt.Felt `cbor:"1,keyasint,omitempty"` // addr -> {key -> value, ...}
	// todo(rdr): felt.Address to felt.Nonce (`Nonce` is a new type that should be created?)
	Nonces map[felt.Felt]*felt.Felt `cbor:"2,keyasint,omitempty"`
	// todo(rdr): felt.Addr to felt.ClassHash (do we know if it will be `SierraClassHash or
	//            `CasmClassHash`)
	DeployedContracts map[felt.Felt]*felt.Felt `cbor:"3,keyasint,omitempty"`
	// todo(rdr): an array of felt.ClassHash, or perhaps, felt.DeprecatedCairoClassHash
	//            Also, change the name from `DeclaredV0Classes` to `DeprecatedDeclaredClasses`
	DeclaredV0Classes []*felt.Felt `cbor:"4,keyasint,omitempty"`
	// todo(rdr): felt.SierraClassHash to felt.CasmClassHash
	DeclaredV1Classes map[felt.Felt]*felt.Felt `cbor:"5,keyasint,omitempty"` // class hash -> compiled class hash
	// todo(rdr): felt.Address to (felt.SierraClassHash or felt.CasmClassHash, I'm unsure)
	ReplacedClasses map[felt.Felt]*felt.Felt `cbor:"6,keyasint,omitempty"` // addr -> class hash
	// Sierra Class definitions which had their compiled class hash definition (CASM)
	// migrated from poseidon hash to blake2s hash (Starknet 0.14.1)
	MigratedClasses map[felt.SierraClassHash]felt.CasmClassHash `cbor:"7,keyasint,omitempty"`
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
	length += len(d.MigratedClasses)

	return uint64(length)
}

func (d *StateDiff) Merge(incoming *StateDiff) {
	// Merge storage diffs
	for addr, newAddrStorage := range incoming.StorageDiffs {
		if oldAddrStorage, exists := d.StorageDiffs[addr]; exists {
			maps.Copy(oldAddrStorage, newAddrStorage)
		} else {
			d.StorageDiffs[addr] = maps.Clone(newAddrStorage)
		}
	}

	// Merge everything else
	maps.Copy(d.Nonces, incoming.Nonces)
	maps.Copy(d.DeployedContracts, incoming.DeployedContracts)
	maps.Copy(d.DeclaredV1Classes, incoming.DeclaredV1Classes)
	maps.Copy(d.ReplacedClasses, incoming.ReplacedClasses)
	maps.Copy(d.MigratedClasses, incoming.MigratedClasses)
	d.DeclaredV0Classes = append(d.DeclaredV0Classes, incoming.DeclaredV0Classes...)
}

// todo(rdr): Is this global variable justified?
var starknetStateDiff0 = felt.FromBytes[felt.Felt]([]byte("STARKNET_STATE_DIFF0"))

func (d *StateDiff) Hash() felt.Felt {
	digest := new(crypto.PoseidonDigest)

	digest.Update(&starknetStateDiff0)

	// updated_contracts = deployedContracts + replacedClasses
	// Digest: [number_of_updated_contracts, address_0, class_hash_0, address_1, class_hash_1, ...].
	updatedContractsDigest(d.DeployedContracts, d.ReplacedClasses, digest)

	// declared classes
	// Digest: [
	//    number_of_declared_classes,
	//    class_hash_0, compiled_class_hash_0,
	//    class_hash_1, compiled_class_hash_1,
	//   ...
	// ]  + [
	//    migrated_class_hash_0, migrated_compiled_class_hash_0,
	//    migrated_class_hash_1, migrated_compiled_class_hash_1,
	//   ...
	// ]
	declaredClassesDigest(d.DeclaredV1Classes, d.MigratedClasses, digest)

	// deprecated_declared_classes
	// Digest: [number_of_old_declared_classes, class_hash_0, class_hash_1, ...].
	deprecatedDeclaredClassesDigest(d.DeclaredV0Classes, digest)

	// Placeholder values
	digest.Update(&felt.One, &felt.Zero)

	// storage_diffs
	// Digest: [
	//	number_of_updated_contracts,
	//  contract_address_0, number_of_updates_in_contract_0, key_0, value0, key1, value1, ...,
	//  contract_address_1, number_of_updates_in_contract_1, key_0, value0, key1, value1, ...,
	// ]
	storageDiffDigest(d.StorageDiffs, digest)

	// nonces
	// Digest: [
	//   number_of_updated_contracts nonces,
	//   contract_address_0, nonce_0,
	//   contract_address_1, nonce_1,
	//   ...,
	// ]
	noncesDigest(d.Nonces, digest)

	/*Poseidon(
	    "STARKNET_STATE_DIFF0",
		deployed_contracts_and_replaced_classes,
		declared_classes,
		deprecated_declared_classes,
	    1,
		0,
		storage_diffs,
		nonces
	)*/
	return digest.Finish()
}

func (d *StateDiff) Commitment() felt.Felt {
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
	// todo(rdr): check if commitment is like this too
	hashOfDeclaredClasses := new(crypto.PoseidonDigest)
	declaredClassesDigest(d.DeclaredV1Classes, d.MigratedClasses, hashOfDeclaredClasses)

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
	deployedContractsHash := hashOfDeployedContracts.Finish()
	declaredClassesHash := hashOfDeclaredClasses.Finish()
	oldDeclaredClassesHash := hashOfOldDeclaredClasses.Finish()
	commitmentDigest := new(crypto.PoseidonDigest)
	commitmentDigest.Update(
		&version,
		&deployedContractsHash,
		&declaredClassesHash,
		&oldDeclaredClassesHash,
	)
	commitmentDigest.Update(tmpFelt.SetUint64(uint64(len(hashOfStorageDomains))))
	for idx := range hashOfStorageDomains {
		storageDomainHash := hashOfStorageDomains[idx].Finish()
		commitmentDigest.Update(
			tmpFelt.SetUint64(uint64(idx)),
			&storageDomainHash,
		)
	}
	return commitmentDigest.Finish()
}

func sortedFeltKeys[V any](m map[felt.Felt]V) []felt.Felt {
	return slices.SortedFunc(maps.Keys(m), func(a, b felt.Felt) int { return a.Cmp(&b) })
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

func declaredClassesDigest(
	declaredClasses map[felt.Felt]*felt.Felt,
	migratedClasses map[felt.SierraClassHash]felt.CasmClassHash,
	digest *crypto.PoseidonDigest,
) {
	mapsLen := len(declaredClasses) + len(migratedClasses)
	numOfDeclaredClasses := felt.FromUint64[felt.Felt](uint64(mapsLen))
	digest.Update(&numOfDeclaredClasses)

	// Sorting the two keys without actually modifying `declaredClasses` or `migratedClasses`
	// to avoid the memory overhead of creating a new map since we **mustn't** mutate them
	keys := make([]felt.Felt, mapsLen)
	index := 0
	for key := range declaredClasses {
		keys[index] = key
		index += 1
	}
	for key := range migratedClasses {
		// todo(rdr): we shouldn't need to convert the key to a felt here but the right fix
		// includes a bigger refactor
		keys[index] = felt.Felt(key)
		index += 1
	}
	slices.SortFunc(keys, func(a felt.Felt, b felt.Felt) int { return a.Cmp(&b) })

	for _, classHash := range keys {
		if casmHash, ok := declaredClasses[classHash]; ok {
			digest.Update(&classHash, casmHash)
		} else {
			casmHash := migratedClasses[felt.SierraClassHash(classHash)]
			digest.Update(&classHash, (*felt.Felt)(&casmHash))
		}
	}
}

func deprecatedDeclaredClassesDigest(declaredV0Classes []*felt.Felt, digest *crypto.PoseidonDigest) {
	numOfDeclaredV0Classes := uint64(len(declaredV0Classes))
	digest.Update(new(felt.Felt).SetUint64(numOfDeclaredV0Classes))

	slices.SortFunc(declaredV0Classes, func(a, b *felt.Felt) int { return a.Cmp(b) })
	digest.Update(declaredV0Classes...)
}

func storageDiffDigest(
	storageDiffs map[felt.Felt]map[felt.Felt]*felt.Felt,
	digest *crypto.PoseidonDigest,
) {
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

func EmptyStateDiff() StateDiff {
	return StateDiff{
		StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt),
		Nonces:            make(map[felt.Felt]*felt.Felt),
		DeployedContracts: make(map[felt.Felt]*felt.Felt),
		DeclaredV0Classes: []*felt.Felt{},
		DeclaredV1Classes: make(map[felt.Felt]*felt.Felt),
		ReplacedClasses:   make(map[felt.Felt]*felt.Felt),
		MigratedClasses:   make(map[felt.SierraClassHash]felt.CasmClassHash),
	}
}

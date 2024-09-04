package core

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

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

func (sd *StateDiff) Print() {
	fmt.Println("StateDiff {")
	fmt.Println("  StorageDiffs:")
	for addr, keyValueMap := range sd.StorageDiffs {
		fmt.Printf("    %s:\n", addr.String())
		for key, value := range keyValueMap {
			fmt.Printf("      %s: %s\n", key.String(), value.String())
		}
	}
	fmt.Println("  Nonces:")
	for addr, nonce := range sd.Nonces {
		fmt.Printf("    %s: %s\n", addr.String(), nonce.String())
	}
	fmt.Println("  DeployedContracts:")
	for addr, classHash := range sd.DeployedContracts {
		fmt.Printf("    %s: %s\n", addr.String(), classHash.String())
	}
	fmt.Println("  DeclaredV0Classes:")
	for _, classHash := range sd.DeclaredV0Classes {
		fmt.Printf("    %s\n", classHash.String())
	}
	fmt.Println("  DeclaredV1Classes:")
	for classHash, compiledClassHash := range sd.DeclaredV1Classes {
		fmt.Printf("    %s: %s\n", classHash.String(), compiledClassHash.String())
	}
	fmt.Println("  ReplacedClasses:")
	for addr, classHash := range sd.ReplacedClasses {
		fmt.Printf("    %s: %s\n", addr.String(), classHash.String())
	}
	fmt.Println("}")
}

func (sd *StateDiff) Diff(other *StateDiff, map1Tag, map2Tag string) (string, bool) {
	var sb strings.Builder
	differencesFound := false

	checkDiff := func(label string, diffFunc func() (string, bool)) {
		sb.WriteString(fmt.Sprintf("  %s:\n", label))
		diffStr, found := diffFunc()
		if found {
			differencesFound = true
			sb.WriteString(diffStr)
		}
	}

	checkDiff("StorageDiffs", func() (string, bool) {
		return compareMapsOfMaps(sd.StorageDiffs, other.StorageDiffs, map1Tag, map2Tag)
	})

	checkDiff("Nonces", func() (string, bool) {
		return compareMaps(sd.Nonces, other.Nonces)
	})

	checkDiff("DeployedContracts", func() (string, bool) {
		return compareMaps(sd.DeployedContracts, other.DeployedContracts)
	})

	checkDiff("DeclaredV0Classes", func() (string, bool) {
		return compareSlices(sd.DeclaredV0Classes, other.DeclaredV0Classes)
	})

	checkDiff("DeclaredV1Classes", func() (string, bool) {
		return compareMaps(sd.DeclaredV1Classes, other.DeclaredV1Classes)
	})

	checkDiff("ReplacedClasses", func() (string, bool) {
		return compareMaps(sd.ReplacedClasses, other.ReplacedClasses)
	})

	return sb.String(), differencesFound
}

func compareMapsOfMaps(map1, map2 map[felt.Felt]map[felt.Felt]*felt.Felt, map1Tag, map2Tag string) (string, bool) {
	var result strings.Builder
	differencesFound := false

	// Iterate through the first map
	for addrMap1, innerMap1 := range map1 {
		if innerMap2, exists := map2[addrMap1]; exists {
			for innerKeyMap1, map1Value := range innerMap1 {
				if map2Value, exists := innerMap2[innerKeyMap1]; !exists {
					result.WriteString(fmt.Sprintf("\tAddr '%s': \n\t\tKey '%s' Value '%s' is in %s, but missing in %s.\n", addrMap1.String(), innerKeyMap1.String(), map1Value.String(), map1Tag, map2Tag))
					differencesFound = true
				} else if !reflect.DeepEqual(map1Value, map2Value) {
					result.WriteString(fmt.Sprintf("\tAddr '%s': \n\t\tKey '%s' has different values:\n\t\t\t%s = '%s', %s = '%s'.\n", addrMap1.String(), innerKeyMap1.String(), map1Tag, map1Value.String(), map2Tag, map2Value.String()))
					differencesFound = true
				}
			}
			for innerKeyMap2 := range innerMap2 {
				if _, exists := innerMap1[innerKeyMap2]; !exists {
					result.WriteString(fmt.Sprintf("\tAddr '%s': \n\t\tKey '%s' Value '%s' is in %s, but missing in %s.\n", addrMap1.String(), innerKeyMap2.String(), innerMap2[innerKeyMap2].String(), map2Tag, map1Tag))
					differencesFound = true
				}
			}
		} else {
			result.WriteString(fmt.Sprintf("\tAddr '%s' is missing in %s.\n", addrMap1.String(), map2Tag))
			differencesFound = true
		}
	}

	// Check for keys in the second map that are not in the first map
	for addrMap2 := range map2 {
		if _, exists := map1[addrMap2]; !exists {
			result.WriteString(fmt.Sprintf("\tAddr '%s' is missing in %s.\n", addrMap2.String(), map1Tag))
			differencesFound = true
		}
	}

	// If no differences are found, indicate that the maps are equal
	if !differencesFound {
		result.WriteString("Both maps are equal.\n")
	}

	return result.String(), differencesFound
}

func compareMaps(m1, m2 map[felt.Felt]*felt.Felt) (string, bool) {
	var sb strings.Builder
	differencesFound := false

	for k, v := range m1 {
		if v2, exists := m2[k]; !exists || !reflect.DeepEqual(v, v2) {
			sb.WriteString(fmt.Sprintf("    %s: %s -> %s (changed or missing in second map)\n", k.String(), v.String(), v2.String()))
			differencesFound = true
		}
	}
	for k, v := range m2 {
		if _, exists := m1[k]; !exists {
			sb.WriteString(fmt.Sprintf("    %s: %s (missing in first map)\n", k.String(), v.String()))
			differencesFound = true
		}
	}
	if !differencesFound {
		sb.WriteString("Both maps are equal.\n")
	}
	return sb.String(), differencesFound
}

func compareSlices(s1, s2 []*felt.Felt) (string, bool) {
	var sb strings.Builder
	differencesFound := false

	len1 := len(s1)
	len2 := len(s2)
	maxLen := len1
	if len2 > len1 {
		maxLen = len2
	}
	// Todo : dont do stupid stuff like this
	s1Copy := make([]*felt.Felt, len(s1))
	copy(s1Copy, s1)
	s2Copy := make([]*felt.Felt, len(s2))
	copy(s2Copy, s2)
	slices.SortFunc(s1, func(a, b *felt.Felt) int {
		return a.Cmp(b)
	})
	slices.SortFunc(s2, func(a, b *felt.Felt) int {
		return a.Cmp(b)
	})
	for i := 0; i < maxLen; i++ {
		if i < len1 && i < len2 {
			if !reflect.DeepEqual(s1Copy[i], s2Copy[i]) {
				sb.WriteString(fmt.Sprintf("    %s -> %s (changed)\n", s1Copy[i].String(), s2Copy[i].String()))
				differencesFound = true
			}
		} else if i < len1 {
			sb.WriteString(fmt.Sprintf("    %s (missing in second slice)\n", s1Copy[i].String()))
			differencesFound = true
		} else {
			sb.WriteString(fmt.Sprintf("    %s (missing in first slice)\n", s2Copy[i].String()))
			differencesFound = true
		}
	}
	if !differencesFound {
		sb.WriteString("Both maps are equal.\n")
	}
	return sb.String(), differencesFound
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

package core

import (
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
	StorageDiffs      map[felt.Felt][]StorageDiff
	Nonces            map[felt.Felt]*felt.Felt
	DeployedContracts []AddressClassHashPair
	DeclaredV0Classes []*felt.Felt
	DeclaredV1Classes []DeclaredV1Class
	ReplacedClasses   []AddressClassHashPair
}

type StorageDiff struct {
	Key   *felt.Felt
	Value *felt.Felt
}

type AddressClassHashPair struct {
	Address   *felt.Felt
	ClassHash *felt.Felt
}

type DeclaredV1Class struct {
	ClassHash         *felt.Felt
	CompiledClassHash *felt.Felt
}

func (d *StateDiff) Commitment() *felt.Felt {
	version := felt.Zero
	var tmpFelt felt.Felt

	/*
		hash_of_deployed_contracts=hash([number_of_deployed_contracts, address_1, class_hash_1,
			address_2, class_hash_2, ...])
	*/
	var hashOfDeployedContracts crypto.PoseidonDigest
	deployedReplacedContracts := append([]AddressClassHashPair{}, d.DeployedContracts...)
	deployedReplacedContracts = append(deployedReplacedContracts, d.ReplacedClasses...)
	hashOfDeployedContracts.Update(tmpFelt.SetUint64(uint64(len(deployedReplacedContracts))))
	sort.Slice(deployedReplacedContracts, func(i, j int) bool {
		cmpRes := deployedReplacedContracts[i].Address.Cmp(deployedReplacedContracts[j].Address)
		if cmpRes == -1 {
			return true
		} else if cmpRes == 1 {
			return false
		}
		return deployedReplacedContracts[i].ClassHash.Cmp(deployedReplacedContracts[j].ClassHash) == -1
	})
	for idx := range deployedReplacedContracts {
		hashOfDeployedContracts.Update(deployedReplacedContracts[idx].Address, deployedReplacedContracts[idx].ClassHash)
	}

	/*
		hash_of_declared_classes = hash([number_of_declared_classes, class_hash_1, compiled_class_hash_1,
			class_hash_2, compiled_class_hash_2, ...])
	*/
	var hashOfDeclaredClasses crypto.PoseidonDigest
	hashOfDeclaredClasses.Update(tmpFelt.SetUint64(uint64(len(d.DeclaredV1Classes))))
	sort.Slice(d.DeclaredV1Classes, func(i, j int) bool {
		return d.DeclaredV1Classes[i].ClassHash.Cmp(d.DeclaredV1Classes[j].ClassHash) == -1
	})
	for idx := range d.DeclaredV1Classes {
		hashOfDeclaredClasses.Update(d.DeclaredV1Classes[idx].ClassHash, d.DeclaredV1Classes[idx].CompiledClassHash)
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

	sortedStorageDiffKeys := sortedFeltKeys(d.StorageDiffs)
	hashOfStorageDomains[daModeL1].Update(tmpFelt.SetUint64(uint64(len(sortedStorageDiffKeys))))
	for idx := range sortedStorageDiffKeys {
		hashOfStorageDomains[daModeL1].Update(&sortedStorageDiffKeys[idx])
		diff := d.StorageDiffs[sortedStorageDiffKeys[idx]]
		sort.Slice(diff, func(i, j int) bool {
			return diff[i].Key.Cmp(diff[j].Key) == -1
		})

		hashOfStorageDomains[daModeL1].Update(tmpFelt.SetUint64(uint64(len(diff))))
		for idx := range diff {
			hashOfStorageDomains[daModeL1].Update(diff[idx].Key, diff[idx].Value)
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

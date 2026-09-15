package main

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/starknet"
)

type contractAddressesResponse struct {
	Starknet eth.Address `json:"Starknet"`
}

func coreContractAddress(gzipped []byte) (eth.Address, error) {
	response, err := unmarshalGzipped[contractAddressesResponse](gzipped)
	if err != nil {
		return eth.Address{}, fmt.Errorf("%s: %w", contractAddresses.name, err)
	}
	return response.Starknet, nil
}

func hexAddress(address eth.Address) string {
	return fmt.Sprintf("0x%x", address.Bytes())
}

type blockInfo struct {
	number      uint64
	timestamp   uint64
	classHashes []string
}

func newBlockInfo(gzipped []byte, number uint64) (blockInfo, error) {
	response, err := decodeStateUpdateResponse(gzipped)
	if err != nil {
		return blockInfo{}, fmt.Errorf("%s %d: %w", stateUpdate.name, number, err)
	}
	return blockInfo{
		number:      response.Block.Number,
		timestamp:   response.Block.Timestamp,
		classHashes: classHashes(&response.StateUpdate.StateDiff),
	}, nil
}

func decodeStateUpdateResponse(gzipped []byte) (*starknet.StateUpdateWithBlockAndSignature, error) {
	response, err := unmarshalGzipped[starknet.StateUpdateWithBlockAndSignature](gzipped)
	if err != nil {
		return nil, err
	}
	if response.Block == nil || response.StateUpdate == nil {
		return nil, errors.New("state update response lacks block or state_update")
	}
	return &response, nil
}

func classHashes(diff *starknet.StateDiff) []string {
	size := len(diff.DeployedContracts) + len(diff.OldDeclaredContracts) + len(diff.DeclaredClasses)
	hashes := make([]string, 0, size)

	for _, deployed := range diff.DeployedContracts {
		hashes = append(hashes, deployed.ClassHash.String())
	}
	for _, hash := range diff.OldDeclaredContracts {
		hashes = append(hashes, hash.String())
	}
	for _, declared := range diff.DeclaredClasses {
		hashes = append(hashes, declared.ClassHash.String())
	}
	return hashes
}

func uniqueClassHashes(blocks []blockInfo) []string {
	set := make(map[string]struct{})
	for _, info := range blocks {
		for _, hash := range info.classHashes {
			set[hash] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(set))
}

func isSierra(classGzipped []byte, hash string) (bool, error) {
	class, err := unmarshalGzipped[starknet.ClassDefinition](classGzipped)
	if err != nil {
		return false, fmt.Errorf("%s %s: %w", classByHash.name, hash, err)
	}
	return class.Sierra != nil, nil
}

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type storageProofArgs struct {
	blockRangeFlags
	NumClasses   int `json:"numClasses"`
	NumContracts int `json:"numContracts"`
	NumKeys      int `json:"numKeys"`
}

func (a *storageProofArgs) bind(cmd *cobra.Command, client *rpcClient) {
	cmd.Flags().IntVar(
		&a.NumClasses,
		"num-classes",
		1,
		"Number of class hashes per request.",
	)
	cmd.Flags().IntVar(
		&a.NumContracts,
		"num-contracts",
		1,
		"Number of contract addresses per request.",
	)
	cmd.Flags().IntVar(
		&a.NumKeys,
		"num-keys",
		1,
		"Number of contract storage keys per request.",
	)
	chainPreRunE(cmd, func() error {
		if a.NumClasses < 0 || a.NumContracts < 0 || a.NumKeys < 0 {
			return fmt.Errorf("--num-classes, --num-contracts and --num-keys must be >= 0")
		}
		return nil
	})
	a.blockRangeFlags.bind(cmd, client)
}

func storageProofSampler(input samplerInput[storageProofArgs]) (storageProofParams, error) {
	// Proofs are only served near the chain head, so query "latest";
	// sampled trie members persist, so historical diffs are still valid sources.
	params := storageProofParams{BlockID: "latest"}
	blockArgs := blockIDArgs{
		blockRangeFlags: input.args.blockRangeFlags,
		BlockIDKind:     blockIDNumber,
	}
	blockInput := input.rebindArgs(&blockArgs)
	storageInput := input.rebindArgs(&storageAtArgs{blockIDArgs: blockArgs})
	for range input.args.NumClasses {
		// Cairo 0 hashes are fine here: keys absent from the classes trie
		// yield valid non-membership proofs with near-identical node work.
		class, err := resample(func() (classAtBlockParams, error) {
			return classAtBlockSampler(blockInput)
		})
		if err != nil {
			return storageProofParams{}, fmt.Errorf("sample class hash: %w", err)
		}
		params.ClassHashes = append(params.ClassHashes, class.ClassHash)
	}
	sampleContract := contractAddressSampler(storageDiffAddresses)
	for range input.args.NumContracts {
		contract, err := resample(func() (contractAtBlockParams, error) {
			return sampleContract(blockInput)
		})
		if err != nil {
			return storageProofParams{}, fmt.Errorf("sample contract address: %w", err)
		}
		params.ContractAddresses = append(params.ContractAddresses, contract.ContractAddress)
	}
	keyIndex := make(map[string]int)
	for range input.args.NumKeys {
		entry, err := resample(func() (storageAtParams, error) {
			return storageAtSampler(storageInput)
		})
		if err != nil {
			return storageProofParams{}, fmt.Errorf("sample storage key: %w", err)
		}
		if i, ok := keyIndex[entry.ContractAddress]; ok {
			params.ContractsStorageKeys[i].StorageKeys = append(
				params.ContractsStorageKeys[i].StorageKeys,
				entry.Key,
			)
			continue
		}
		keyIndex[entry.ContractAddress] = len(params.ContractsStorageKeys)
		params.ContractsStorageKeys = append(
			params.ContractsStorageKeys,
			contractStorageKeys{
				ContractAddress: entry.ContractAddress,
				StorageKeys:     []string{entry.Key},
			},
		)
	}
	return params, nil
}

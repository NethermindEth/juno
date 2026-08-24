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

func storageProofExtraArgs(cmd *cobra.Command, args *storageProofArgs) {
	addBlockRangeFlags(cmd, &args.blockRangeFlags)
	cmd.Flags().IntVar(
		&args.NumClasses,
		"num-classes",
		1,
		"Number of class hashes per request.",
	)
	cmd.Flags().IntVar(
		&args.NumContracts,
		"num-contracts",
		1,
		"Number of contract addresses per request.",
	)
	cmd.Flags().IntVar(
		&args.NumKeys,
		"num-keys",
		1,
		"Number of contract storage keys per request.",
	)
	chainPreRunE(cmd, func() error {
		if args.NumClasses < 0 || args.NumContracts < 0 || args.NumKeys < 0 {
			return fmt.Errorf("--num-classes, --num-contracts and --num-keys must be >= 0")
		}
		return nil
	})
}

func storageProofSampler(input samplerInput[storageProofArgs]) (any, error) {
	// Proofs are only served near the chain head, so query "latest";
	// sampled trie members persist, so historical diffs are still valid sources.
	params := storageProofParams{BlockID: "latest"}
	for range input.args.NumClasses {
		// Cairo 0 hashes are fine here: keys absent from the classes trie
		// yield valid non-membership proofs with near-identical node work.
		classHash, err := resample(func() (string, error) {
			return sampleClassHash(input, input.args.sampleBlockNumber(input.rng))
		})
		if err != nil {
			return nil, fmt.Errorf("sample class hash: %w", err)
		}
		params.ClassHashes = append(params.ClassHashes, classHash)
	}
	for range input.args.NumContracts {
		address, err := resample(func() (string, error) {
			return sampleContractAddress(
				input,
				input.args.sampleBlockNumber(input.rng),
				storageDiffAddresses,
			)
		})
		if err != nil {
			return nil, fmt.Errorf("sample contract address: %w", err)
		}
		params.ContractAddresses = append(params.ContractAddresses, address)
	}
	keyIndex := make(map[string]int)
	for range input.args.NumKeys {
		entry, err := resample(func() (storageEntry, error) {
			return sampleStorageEntry(input, input.args.sampleBlockNumber(input.rng))
		})
		if err != nil {
			return nil, fmt.Errorf("sample storage key: %w", err)
		}
		if i, ok := keyIndex[entry.Address]; ok {
			params.ContractsStorageKeys[i].StorageKeys = append(
				params.ContractsStorageKeys[i].StorageKeys,
				entry.Key,
			)
			continue
		}
		keyIndex[entry.Address] = len(params.ContractsStorageKeys)
		params.ContractsStorageKeys = append(
			params.ContractsStorageKeys,
			contractStorageKeys{ContractAddress: entry.Address, StorageKeys: []string{entry.Key}},
		)
	}
	return params, nil
}

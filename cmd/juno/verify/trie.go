package verify

import (
	"fmt"
	"slices"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	verifytrie "github.com/NethermindEth/juno/verify/trie"
	"github.com/spf13/cobra"
)

const (
	verifyTrieType     = "type"
	verifyContractAddr = "address"
)

func verifyTrieCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "trie",
		Short:         "Verify trie integrity",
		Long:          `Verify trie integrity by rebuilding tries and comparing root hashes.`,
		RunE:          runTrieVerify,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().StringSlice(
		verifyTrieType,
		nil,
		"Trie types to verify (contract, class, contract-storage)."+
			"If not specified, all trie types are verified.",
	)

	cmd.Flags().String(
		verifyContractAddr,
		"",
		"Contract address to verify (only used with --type contract-storage). "+
			"If not specified, all contract storage tries are verified.",
	)

	return cmd
}

func runTrieVerify(cmd *cobra.Command, args []string) error {
	dbPath, err := cmd.Flags().GetString(verifyDBPathF)
	if err != nil {
		return err
	}

	database, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	trieTypes, err := cmd.Flags().GetStringSlice(verifyTrieType)
	if err != nil {
		return err
	}

	contractAddrStr, err := cmd.Flags().GetString(verifyContractAddr)
	if err != nil {
		return err
	}

	var tries []verifytrie.TrieType
	if len(trieTypes) > 0 {
		tries = make([]verifytrie.TrieType, 0, len(trieTypes))
		for _, t := range trieTypes {
			tt := verifytrie.TrieType(t)
			if !tt.IsValid() {
				return fmt.Errorf("invalid trie type %q (allowed: contract, class, contract-storage)", t)
			}
			tries = append(tries, tt)
		}
	}

	var contractAddr *felt.Felt
	if contractAddrStr != "" {
		hasContractStorage := slices.Contains(tries, verifytrie.ContractStorageTrie)
		if len(tries) == 0 {
			hasContractStorage = true
		}

		if !hasContractStorage {
			return fmt.Errorf("--address flag can only be used with --type contract-storage")
		}

		var addr felt.Felt
		_, err = addr.SetString(contractAddrStr)
		if err != nil {
			return fmt.Errorf("invalid contract address %s: %w", contractAddrStr, err)
		}
		contractAddr = &addr
	}

	logLevel := utils.NewLogLevel(utils.INFO)
	logger, err := utils.NewZapLogger(logLevel, true)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	verifier := verifytrie.NewTrieVerifier(database, logger, tries, contractAddr)
	ctx := cmd.Context()
	return verifier.Run(ctx)
}

package verify

import (
	"github.com/spf13/cobra"
)

const (
	verifyDBPathF  = "db-path"
	verifyTrieType = "type"
)

func VerifyCmd(defaultDBPath string) *cobra.Command {
	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify database integrity",
		Long:  `Verify database integrity using various verification methods.`,
	}

	verifyCmd.PersistentFlags().String(verifyDBPathF, defaultDBPath, "Path to the database")
	verifyCmd.AddCommand(verifyTrieCmd())
	verifyCmd.RunE = verifyAll

	return verifyCmd
}

// verifyAll runs all verifiers with default scope when no subcommand is specified.
func verifyAll(cmd *cobra.Command, args []string) error {
	dbPath, err := cmd.Flags().GetString(verifyDBPathF)
	if err != nil {
		return err
	}

	database, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	trieVerifier := NewTrieVerifier(database)

	runner := &VerifyRunner{
		Verifiers: []Verifier{
			trieVerifier,
		},
	}

	configs := make(map[string]Config)
	ctx := cmd.Context()
	return runner.Run(ctx, configs)
}

func verifyTrieCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trie",
		Short: "Verify trie integrity",
		Long:  `Verify trie integrity by rebuilding tries from leaf nodes and comparing root hashes.`,
		RunE:  runTrieVerify,
	}

	cmd.Flags().StringSlice(verifyTrieType, nil, "Trie types to verify (state, class, contract). Can be specified multiple times. Empty = all.")

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

	cfg := &TrieConfig{}

	if len(trieTypes) > 0 {
		cfg.Tries = make([]TrieType, len(trieTypes))
		for i, t := range trieTypes {
			cfg.Tries[i] = TrieType(t)
		}
	}

	verifier := NewTrieVerifier(database)
	ctx := cmd.Context()
	return verifier.Run(ctx, cfg)
}

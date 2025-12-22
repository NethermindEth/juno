package verify

import (
	"errors"
	"fmt"
	"os"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/utils"
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

	logLevel := utils.NewLogLevel(utils.INFO)
	logger, err := utils.NewZapLogger(logLevel, true)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	trieVerifier := NewTrieVerifier(database, logger)

	verifier := &VerifyRunner{
		Verifiers: []Verifier{
			trieVerifier,
		},
	}

	ctx := cmd.Context()
	return verifier.Run(ctx, nil)
}

func openDB(path string) (db.KeyValueStore, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, errors.New("database path does not exist")
	}

	database, err := pebblev2.New(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	return database, nil
}

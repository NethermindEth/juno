package verify

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/verify/trie"
	"github.com/spf13/cobra"
)

type Verifier interface {
	Name() string
	Run(ctx context.Context) error
}

const verifyDBPathF = "db-path"

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

	ctx := cmd.Context()

	verifiers := []Verifier{
		trie.NewTrieVerifier(database, logger, nil, nil),
	}

	for _, v := range verifiers {
		if err := v.Run(ctx); err != nil {
			return fmt.Errorf("%s verification stopped: %w", v.Name(), err)
		}
	}

	return nil
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

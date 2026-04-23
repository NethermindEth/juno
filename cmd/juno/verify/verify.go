package verify

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
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

	return verifyCmd
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

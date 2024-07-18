package main

import (
	"fmt"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/spf13/cobra"
)

func DBSize() *cobra.Command {
	dbSizeCmd := &cobra.Command{
		Use:   "db-size",
		Short: "Calculate's Juno's DB size.",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, err := cmd.Flags().GetString(dbPathF)
			if err != nil {
				return err
			}

			if dbPath == "" {
				return fmt.Errorf("--%v cannot be empty", dbPathF)
			}

			pebbleDB, err := pebble.New(dbPath)
			if err != nil {
				return err
			}

			var totalSize, stateSizeWithoutHistory, stateSizeWithHistory uint

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Total number of DB buckets:", len(db.BucketValues()))
			if err != nil {
				return err
			}

			var bucketSize uint
			for _, b := range db.BucketValues() {
				bucketSize, err = pebble.CalculatePrefixSize(cmd.Context(), pebbleDB.(*pebble.DB), []byte{byte(b)})
				if err != nil {
					return err
				}

				_, err = fmt.Fprintln(cmd.OutOrStdout(), uint(b)+1, "Size of", b, "=", bucketSize)
				if err != nil {
					return err
				}

				totalSize += bucketSize

				if utils.AnyOf(b, db.StateTrie, db.ContractStorage, db.Class, db.ContractNonce,
					db.ContractDeploymentHeight) {
					stateSizeWithoutHistory += bucketSize
					stateSizeWithHistory += bucketSize
				}

				if utils.AnyOf(b, db.ContractStorageHistory, db.ContractNonceHistory, db.ContractClassHashHistory) {
					stateSizeWithHistory += bucketSize
				}
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "State size without history =", stateSizeWithoutHistory)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "State size with history =", stateSizeWithHistory)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Total DB size =", totalSize)
			return err
		},
	}

	// Persistent Flag was not used from the Juno command because GenP2PKeyPair would also inherit it while PersistentPreRun was not used
	// because none of the subcommand required access to the node.Config.
	defaultDBPath, dbPathShort := "", "p"
	dbSizeCmd.Flags().StringP(dbPathF, dbPathShort, defaultDBPath, dbPathUsage)

	return dbSizeCmd
}

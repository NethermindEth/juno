package main

import (
	"fmt"

	"github.com/NethermindEth/juno/utils"
	"github.com/spf13/cobra"
)


func NewSnapshotCmd() *cobra.Command{
	var location, network string

	var snapshotCommand = &cobra.Command{
	Use:   "snapshot [flags]",
	Short: "Download snapshot based on network",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if err := utils.DownloadFile(ctx, network, location, &utils.RealDownloader{}); err != nil {
			return err
		}
		fmt.Printf("The %s snapshot was successfully downloaded to %s\nYou can run juno with the flag `--db-path` to use the downloaded snapshot\n", network, location) //nolint:lll
		
		return nil
	},
}
	snapshotCommand.Flags().StringVar(&location, "location", "l", "Location to where the snapshot will be saved")
	snapshotCommand.Flags().StringVar(&network, "network", "n", "Network (mainnet/goerli/goerli2)")
	snapshotCommand.MarkFlagRequired("location") //nolint
	snapshotCommand.MarkFlagRequired("network") //nolint

	return snapshotCommand
}

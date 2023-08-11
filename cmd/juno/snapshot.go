package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/NethermindEth/juno/utils"
	"github.com/spf13/cobra"
)

var location, network string

var snapshotCommand = &cobra.Command{
	Use:   "snapshot [flags]",
	Short: "Download snapshot based on network",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			quiteDownload := make(chan os.Signal, 1)
			signal.Notify(quiteDownload, os.Interrupt, syscall.SIGTERM)
			<-quiteDownload
			cancel()
		}()
		downloadError := utils.DownloadFile(ctx, network, location, &utils.RealDownloader{})
		if downloadError != nil {
			return downloadError
		} else {
			fmt.Printf("The %s snapshot was successfully downloaded to %s\nYou can run juno with the flag `--db-path` to use the downloaded snapshot\n", network, location) //nolint:lll
		}
		return nil
	},
}

func SnapshotInit(junoCmd *cobra.Command) {
	snapshotCommand.Flags().StringVar(&location, "location", "l", "Location to where the snapshot will be saved")
	snapshotCommand.Flags().StringVar(&network, "network", "n", "Network (mainnet/goerli/goerli2)")
	snapshotCommand.MarkFlagRequired("location") //nolint
	snapshotCommand.MarkFlagRequired("network") //nolint

	junoCmd.AddCommand(snapshotCommand)
}

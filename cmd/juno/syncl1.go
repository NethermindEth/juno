package main

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newSyncL1Cmd(endBlock *uint64, run func(*cobra.Command, []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "syncl1 <end-block-number> [flags]",
		Short: "sync the state from L1 using Juno's database format.",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			var err error
			*endBlock, err = strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("parse end block number: %v", err)
			}
			return nil
		},
		RunE: run,
	}
	return cmd
}

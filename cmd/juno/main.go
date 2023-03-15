package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"github.com/spf13/cobra"
)

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	cmd, n := NewCmd()

	cmd.Run = func(cmd *cobra.Command, _ []string) {
		ctx, cancel := context.WithCancel(cmd.Context())
		go func() {
			<-quit
			cancel()
		}()
		n.Run(ctx)
	}

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

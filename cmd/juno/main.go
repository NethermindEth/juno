package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/NethermindEth/juno/node"
)

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-quit
		cancel()
	}()

	if err := NewCmd(node.New).ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

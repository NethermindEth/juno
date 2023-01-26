package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/NethermindEth/juno/node"
)

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	if err := NewCmd(node.New, quit).Execute(); err != nil {
		os.Exit(1)
	}
}

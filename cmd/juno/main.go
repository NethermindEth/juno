package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/NethermindEth/juno/internal/node"
)

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	if err := NewCmd(node.New, quit).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

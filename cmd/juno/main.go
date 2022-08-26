package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/NethermindEth/juno/internal/juno"
)

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	if err := NewCmd(juno.New, quit).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

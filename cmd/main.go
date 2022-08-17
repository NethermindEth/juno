package main

import (
	"fmt"
	"os"

	junoCmd "github.com/NethermindEth/juno/cmd/juno"
	"github.com/NethermindEth/juno/internal/juno"
)

func main() {
	if err := junoCmd.NewCmd(juno.New).Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err.Error())
		os.Exit(1)
	}
}

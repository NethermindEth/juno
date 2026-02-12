package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/spf13/cobra"
)

func CompileSierraCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compile-sierra",
		Short: "Compile a Sierra class to CASM via stdin/stdout.",
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}

			var sierra starknet.SierraClass
			if err := json.Unmarshal(input, &sierra); err != nil {
				return fmt.Errorf("unmarshal sierra class: %w", err)
			}

			casmClass, err := compiler.CompileFFI(&sierra)
			if err != nil {
				return err
			}

			output, err := json.Marshal(casmClass)
			if err != nil {
				return fmt.Errorf("marshal casm class: %w", err)
			}

			_, err = cmd.OutOrStdout().Write(output)
			return err
		},
	}
}

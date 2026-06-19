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
	var maxMemory, maxCPUTime uint64

	cmd := &cobra.Command{
		Use:   "compile-sierra",
		Short: "Compile a Sierra class to CASM via stdin/stdout.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Apply resource limits to ourselves before any compilation
			// work, so the (untrusted) compile runs fully bounded with no
			// race window. Enforced on Linux only; a no-op elsewhere.
			if err := compiler.ApplySelfRLimits(maxCPUTime, maxMemory); err != nil {
				return fmt.Errorf("applying compilation resource limits: %w", err)
			}

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

	cmd.Flags().Uint64Var(&maxMemory, compiler.FlagMaxMemory, 0,
		"Address-space limit (RLIMIT_AS) in bytes for the compilation; 0 disables it. Linux only.")
	cmd.Flags().Uint64Var(&maxCPUTime, compiler.FlagMaxCPUTime, 0,
		"CPU-time limit (RLIMIT_CPU) in seconds for the compilation; 0 disables it. Linux only.")

	return cmd
}

package verify

import (
	"context"
	"fmt"
)

// Config is the base configuration interface that all verifier configs must implement.
// Empty values in config structs imply default behavior.
type Config interface{}

// Verifier defines the interface for all database verifiers.
// Each verifier is atomic, has its own strongly-typed config, and contains no CLI logic.
type Verifier interface {
	// Name returns the name of the verifier (e.g., "trie", "tx").
	Name() string
	// DefaultConfig returns the default configuration for this verifier.
	DefaultConfig() Config
	// Run executes the verification with the given config and context.
	// Returns an error if verification fails.
	Run(ctx context.Context, cfg Config) error
}

// VerifyRunner orchestrates the execution of multiple verifiers.
type VerifyRunner struct {
	Verifiers []Verifier
}

// Run executes all registered verifiers sequentially.
// Stops on the first error and wraps it with the verifier name.
func (r *VerifyRunner) Run(ctx context.Context, configs map[string]Config) error {
	for _, verifier := range r.Verifiers {
		cfg, ok := configs[verifier.Name()]
		if !ok {
			cfg = verifier.DefaultConfig()
		}

		if err := verifier.Run(ctx, cfg); err != nil {
			return fmt.Errorf("%s: %w", verifier.Name(), err)
		}
	}
	return nil
}

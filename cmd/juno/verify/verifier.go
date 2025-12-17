package verify

import (
	"context"
	"fmt"
)

type Config interface{}

type Verifier interface {
	Name() string
	DefaultConfig() Config
	Run(ctx context.Context, cfg Config) error
}

type VerifyRunner struct {
	Verifiers []Verifier
}

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

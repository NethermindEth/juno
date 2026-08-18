package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/spf13/cobra"
)

const maxResampleAttempts = 100

type samplerInput[T any] struct {
	ctx    context.Context
	client *rpcClient
	rng    *rand.Rand
	args   *T
	cache  *sierraCache
}

type sampler[T any] func(input samplerInput[T]) (any, error)

func newSampledCmds(
	cfg *rootConfig,
	samplers map[string]sampler[blockRangeFlags],
) []*cobra.Command {
	cmds := make([]*cobra.Command, 0, len(samplers))
	for method, sample := range samplers {
		cmds = append(cmds, newSampledCmd(cfg, method, sample, addBlockRangeFlags))
	}
	return cmds
}

// newSampledCmd builds a subcommand whose params come from one successful call
// of sample (errResample re-invokes); extraArgs fills *T, the corpus sampling meta.
func newSampledCmd[T any](
	cfg *rootConfig,
	method string,
	sample sampler[T],
	extraArgs func(cmd *cobra.Command, args *T),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     commandName(method),
		Aliases: []string{method},
		GroupID: methodsGroupID,
	}
	args := new(T)
	extraArgs(cmd, args)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		client := newRPCClient(cfg.sourceURL, cfg.concurrency)
		if resolver, ok := any(args).(blockRangeResolver); ok {
			if err := resolver.resolveBlockRange(cmd.Context(), client); err != nil {
				return err
			}
		}
		cache := newSierraCache()
		gen := func(ctx context.Context, client *rpcClient, rng *rand.Rand) (any, error) {
			input := samplerInput[T]{
				ctx:    ctx,
				client: client,
				rng:    rng,
				args:   args,
				cache:  cache,
			}
			result, err := resample(func() (any, error) { return sample(input) })
			if err != nil {
				return nil, fmt.Errorf("%s: %w", method, err)
			}
			return result, nil
		}
		return runCorpus(cmd, cfg, client, method, args, gen)
	}
	return cmd
}

// errResample tells resample to retry fn with fresh draws.
var errResample = errors.New("resample")

// resample retries fn until it succeeds or fails with a non-errResample error.
func resample[R any](fn func() (R, error)) (R, error) {
	var zero R
	for range maxResampleAttempts {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, errResample) {
			return zero, err
		}
	}
	return zero, fmt.Errorf("no candidate found after %d attempts", maxResampleAttempts)
}

func commandName(method string) string {
	return strings.TrimPrefix(method, "starknet_")
}

func chainPreRunE(cmd *cobra.Command, check func() error) {
	prev := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		if prev != nil {
			if err := prev(c, args); err != nil {
				return err
			}
		}
		return check()
	}
}

// pickRandom returns a random element of items, or errResample when empty.
func pickRandom[T any](rng *rand.Rand, items []T) (T, error) {
	var zero T
	if len(items) == 0 {
		return zero, errResample
	}
	return items[rng.Uint64N(uint64(len(items)))], nil
}

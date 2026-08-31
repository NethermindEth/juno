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

type sampler[T, R any] func(input samplerInput[T]) (R, error)

// argsBinder registers an args type's flags and PreRunE checks on a command.
type argsBinder interface {
	bind(cmd *cobra.Command, client *rpcClient)
}

func (input samplerInput[T]) rebindArgs[U any](args *U) samplerInput[U] {
	return samplerInput[U]{
		ctx:    input.ctx,
		client: input.client,
		rng:    input.rng,
		args:   args,
		cache:  input.cache,
	}
}

// newSampledCmd builds a subcommand whose params come from one successful call
// of sample (errResample re-invokes); T's bind fills *T, the corpus sampling meta.
func (cfg *rootConfig) newSampledCmd[T any, PT interface {
	*T
	argsBinder
}, R any](
	method string,
	sample sampler[T, R],
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     commandName(method),
		Aliases: []string{method},
		GroupID: methodsGroupID,
	}
	args := new(T)
	client := &rpcClient{}
	chainPreRunE(cmd, func() error {
		*client = *newRPCClient(cfg.sourceURL, cfg.concurrency)
		return nil
	})
	PT(args).bind(cmd, client)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		cache := newSierraCache()
		gen := func(ctx context.Context, rng *rand.Rand) (any, error) {
			input := samplerInput[T]{
				ctx:    ctx,
				client: client,
				rng:    rng,
				args:   args,
				cache:  cache,
			}
			result, err := resample(func() (R, error) { return sample(input) })
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

func uniformRange(rng *rand.Rand, minValue, maxValue uint64) uint64 {
	return minValue + rng.Uint64N(maxValue-minValue+1)
}

// pickRandom returns a random element of items, or errResample when empty.
func pickRandom[T any](rng *rand.Rand, items []T) (T, error) {
	var zero T
	if len(items) == 0 {
		return zero, errResample
	}
	return items[rng.Uint64N(uint64(len(items)))], nil
}

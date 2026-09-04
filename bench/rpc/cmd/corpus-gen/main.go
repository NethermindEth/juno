package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand/v2"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cobra"
)

const (
	defaultCount     = 1000
	defaultSeed      = 1
	defaultSourceURL = "http://localhost:6060/v0_10"
)

type rootConfig struct {
	count       int
	seed        uint64
	sourceURL   string
	batch       int
	concurrency int
}

func newRootCmd() *cobra.Command {
	cfg := &rootConfig{}

	cmd := &cobra.Command{
		Use:          "corpus-gen",
		Short:        "Generate seeded, RPC-version-agnostic JSON-RPC benchmark corpora.",
		SilenceUsage: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			if cfg.count < 1 {
				return fmt.Errorf("--count must be >= 1 (got %d)", cfg.count)
			}
			if cfg.batch < 0 {
				return fmt.Errorf("--batch must be >= 0 (got %d)", cfg.batch)
			}
			if cfg.concurrency < 1 {
				return fmt.Errorf("--concurrency must be >= 1 (got %d)", cfg.concurrency)
			}
			return nil
		},
	}

	pf := cmd.PersistentFlags()
	pf.IntVar(&cfg.count, "count", defaultCount, "Number of corpus entries to generate.")
	pf.Uint64Var(&cfg.seed, "seed", defaultSeed, "Numeric seed for deterministic sampling.")
	pf.StringVar(
		&cfg.sourceURL,
		"source-url",
		defaultSourceURL,
		"JSON-RPC URL of the source node to sample.",
	)
	pf.IntVar(
		&cfg.batch,
		"batch",
		0,
		"Batch size per entry; omit for plain request objects, N for JSON-RPC arrays of N.",
	)
	pf.IntVar(
		&cfg.concurrency,
		"concurrency",
		runtime.GOMAXPROCS(0),
		"Max concurrent sampling requests to the source node.",
	)

	cmd.AddGroup(&cobra.Group{ID: methodsGroupID, Title: "RPC Methods:"})
	cmd.AddCommand(
		cfg.newSampledCmd("starknet_getBlockWithTxHashes", blockIDSampler),
		cfg.newSampledCmd("starknet_getBlockWithTxs", blockIDWithProofFactsSampler),
		cfg.newSampledCmd("starknet_getBlockWithReceipts", blockIDWithProofFactsSampler),
		cfg.newSampledCmd("starknet_getStateUpdate", blockIDSampler),
		cfg.newSampledCmd("starknet_getBlockTransactionCount", blockIDSampler),
		cfg.newSampledCmd("starknet_traceBlockTransactions", traceBlockTransactionsSampler),
		cfg.newSampledCmd("starknet_getTransactionByHash", txHashWithProofFactsSampler),
		cfg.newSampledCmd("starknet_getTransactionStatus", txHashSampler),
		cfg.newSampledCmd("starknet_getTransactionReceipt", txHashSampler),
		cfg.newSampledCmd("starknet_traceTransaction", txHashSampler),
		cfg.newSampledCmd("starknet_getTransactionByBlockIdAndIndex", txByBlockIDAndIndexSampler),
		cfg.newSampledCmd("starknet_getClassHashAt", contractAddressSampler(storageDiffAddresses)),
		cfg.newSampledCmd("starknet_getClassAt", contractAddressSampler(storageDiffAddresses)),
		cfg.newSampledCmd("starknet_getNonce", contractAddressSampler(nonceAddresses)),
		cfg.newSampledCmd("starknet_getClass", classAtBlockSampler),
		cfg.newSampledCmd("starknet_getCompiledCasm", sierraClassHashSampler),
		cfg.newSampledCmd("starknet_getStorageAt", storageAtSampler),
		cfg.newSampledCmd("starknet_getEvents", eventsSampler),
		cfg.newSampledCmd("starknet_getStorageProof", storageProofSampler),
	)
	cmd.AddCommand(newNoParamCmds(
		cfg,
		"starknet_blockNumber",
		"starknet_blockHashAndNumber",
		"starknet_chainId",
		"starknet_syncing",
	)...)
	return cmd
}

const methodsGroupID = "methods"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

type corpusMeta[T any] struct {
	Method      string `json:"method"`
	RPCVersion  string `json:"rpcVersion"`
	Seed        uint64 `json:"seed"`
	Count       int    `json:"count"`
	Batch       int    `json:"batch"`
	GeneratedAt string `json:"generatedAt"`
	Sampling    T      `json:"sampling"`
}

type corpus[T any] struct {
	Meta     corpusMeta[T] `json:"meta"`
	Requests []any         `json:"requests"`
}

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

func writeCorpus[T any](w io.Writer, c *corpus[T]) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = w.Write(data)
	return err
}

type paramsGen func(ctx context.Context, rng *rand.Rand) (any, error)

func runCorpus[T any](
	cmd *cobra.Command,
	cfg *rootConfig,
	client *rpcClient,
	method string,
	meta T,
	gen paramsGen,
) error {
	generatedAt := time.Now().UTC().Format(time.RFC3339)

	c, err := buildCorpus(
		cmd.Context(), cfg, client, method, meta, generatedAt, gen, cmd.ErrOrStderr(),
	)
	if err != nil {
		return err
	}

	if err := writeCorpus(cmd.OutOrStdout(), c); err != nil {
		return err
	}

	_, err = fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d entries\n", cfg.count)
	return err
}

func buildCorpus[T any](
	ctx context.Context,
	cfg *rootConfig,
	client *rpcClient,
	method string,
	meta T,
	generatedAt string,
	gen paramsGen,
	progress io.Writer,
) (*corpus[T], error) {
	version, err := client.specVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch spec version: %w", err)
	}

	var completed atomic.Int64
	stopProgress := reportProgress(progress, &completed, cfg.count)
	defer stopProgress()

	seed := methodSeed(cfg.seed, method)
	requests := make([]any, cfg.count)
	p := pool.New().
		WithContext(ctx).
		WithMaxGoroutines(cfg.concurrency).
		WithCancelOnError().
		WithFirstError()
	for i := range requests {
		p.Go(func(ctx context.Context) error {
			rng := newSeededRand(seed, uint64(i))
			batchSize := max(1, cfg.batch)
			entry := make([]jsonRPCRequest, batchSize)
			for j := range entry {
				params, genErr := gen(ctx, rng)
				if genErr != nil {
					return genErr
				}
				entry[j] = jsonRPCRequest{
					JSONRPC: "2.0", ID: i*batchSize + j + 1, Method: method, Params: params,
				}
			}
			if cfg.batch == 0 {
				requests[i] = entry[0]
			} else {
				requests[i] = entry
			}
			completed.Add(1)
			return nil
		})
	}
	if err := p.Wait(); err != nil {
		return nil, err
	}

	return &corpus[T]{
		Meta: corpusMeta[T]{
			Method:      method,
			RPCVersion:  version,
			Seed:        cfg.seed,
			Count:       cfg.count,
			Batch:       cfg.batch,
			GeneratedAt: generatedAt,
			Sampling:    meta,
		},
		Requests: requests,
	}, nil
}

const progressInterval = 2 * time.Second

func reportProgress(w io.Writer, completed *atomic.Int64, total int) (stop func()) {
	quit := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(progressInterval)
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				fmt.Fprintf(w, "progress: %d/%d entries\n", completed.Load(), total)
			}
		}
	}()
	return func() {
		close(quit)
		wg.Wait()
	}
}

func newSeededRand(seed, stream uint64) *rand.Rand {
	//nolint:gosec // G404: deterministic corpus needs a seeded PRNG, not crypto randomness
	return rand.New(rand.NewPCG(seed, stream))
}

// methodSeed decorrelates draws across methods; flag variants of one method stay aligned.
func methodSeed(seed uint64, method string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(method))
	return seed ^ h.Sum64()
}

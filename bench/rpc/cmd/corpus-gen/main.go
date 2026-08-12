package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"runtime"
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
	pf.StringVar(&cfg.sourceURL, "source-url", defaultSourceURL,
		"JSON-RPC URL of the source node to sample.")
	pf.IntVar(&cfg.batch, "batch", 0,
		"Batch size per entry; omit for plain request objects, N for JSON-RPC arrays of N.")
	pf.IntVar(&cfg.concurrency, "concurrency", runtime.GOMAXPROCS(0),
		"Max concurrent sampling requests to the source node.")

	cmd.AddCommand(
		newGetTxByHashCmd(cfg),
		newGetTxReceiptCmd(cfg),
		newGetBlockWithTxsCmd(cfg),
		newGetBlockWithTxHashesCmd(cfg),
		newGetBlockWithReceiptsCmd(cfg),
	)
	return cmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

type blockRange struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
}

func addBlockRangeFlags(cmd *cobra.Command, start, end *uint64) {
	cmd.Flags().Uint64Var(start, "block-start", 0,
		"Start block number to sample from (inclusive).")
	cmd.Flags().Uint64Var(end, "block-end", 0,
		"End block number to sample from (exclusive).")
	_ = cmd.MarkFlagRequired("block-start")
	_ = cmd.MarkFlagRequired("block-end")
}

func validateBlockRange(start, end uint64) error {
	if end <= start {
		return fmt.Errorf("--block-end (%d) must be > --block-start (%d)", end, start)
	}
	return nil
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
	Params  any    `json:"params"`
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

type paramsGen func(ctx context.Context, client *rpcClient, rng *rand.Rand) (any, error)

func runCorpus[T any](
	cmd *cobra.Command, cfg *rootConfig, method string, meta T, gen paramsGen,
) error {
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	client := newRPCClient(cfg.sourceURL, cfg.concurrency)

	c, err := buildCorpus(cmd.Context(), cfg, client, method, meta, generatedAt, gen)
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
	ctx context.Context, cfg *rootConfig, client *rpcClient,
	method string, meta T, generatedAt string, gen paramsGen,
) (*corpus[T], error) {
	version, err := client.specVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch spec version: %w", err)
	}

	requests := make([]any, cfg.count)
	p := pool.New().
		WithContext(ctx).
		WithMaxGoroutines(cfg.concurrency).
		WithCancelOnError().
		WithFirstError()
	for i := range requests {
		p.Go(func(ctx context.Context) error {
			rng := newSeededRand(cfg.seed, uint64(i))
			batchSize := max(1, cfg.batch)
			entry := make([]jsonRPCRequest, batchSize)
			for j := range entry {
				params, genErr := gen(ctx, client, rng)
				if genErr != nil {
					return genErr
				}
				entry[j] = jsonRPCRequest{
					JSONRPC: "2.0", ID: i*batchSize + j + 1, Method: method, Params: params,
				}
			}
			if cfg.batch == 0 {
				requests[i] = entry[0]
				return nil
			}
			requests[i] = entry
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

func newSeededRand(seed, stream uint64) *rand.Rand {
	//nolint:gosec // G404: deterministic corpus needs a seeded PRNG, not crypto randomness
	return rand.New(rand.NewPCG(seed, stream))
}

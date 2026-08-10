package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultCount     = 1000
	defaultSeed      = 1
	defaultSourceURL = "http://localhost:6060/v0_10"
)

type rootConfig struct {
	count     int
	seed      uint64
	sourceURL string
	batch     int
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

	cmd.AddCommand(newGetTxByHashCmd(cfg))
	cmd.AddCommand(newGetTxReceiptCmd(cfg))
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

func runCorpus[T any](
	cmd *cobra.Command, cfg *rootConfig, method string, meta T,
	makeGen func(*rpcClient, *rand.Rand) func() (any, error),
) error {
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	client := newRPCClient(cfg.sourceURL)
	rng := newSeededRand(cfg.seed)

	c, err := buildCorpus(cmd.Context(), cfg, client, method, meta, generatedAt, makeGen(client, rng))
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
	method string, meta T, generatedAt string, nextParams func() (any, error),
) (*corpus[T], error) {
	version, err := client.specVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch spec version: %w", err)
	}

	id := 0
	nextRequest := func() (jsonRPCRequest, error) {
		params, paramsErr := nextParams()
		if paramsErr != nil {
			return jsonRPCRequest{}, paramsErr
		}
		id++
		return jsonRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}, nil
	}

	requests := make([]any, 0, cfg.count)
	for range cfg.count {
		if cfg.batch < 1 {
			req, reqErr := nextRequest()
			if reqErr != nil {
				return nil, reqErr
			}
			requests = append(requests, req)
			continue
		}
		entry := make([]jsonRPCRequest, 0, cfg.batch)
		for range cfg.batch {
			req, reqErr := nextRequest()
			if reqErr != nil {
				return nil, reqErr
			}
			entry = append(entry, req)
		}
		requests = append(requests, entry)
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

func newSeededRand(seed uint64) *rand.Rand {
	//nolint:gosec // G404: deterministic corpus needs a seeded PRNG, not crypto randomness
	return rand.New(rand.NewPCG(seed, 0))
}

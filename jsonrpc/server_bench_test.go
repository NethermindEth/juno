package jsonrpc_test

import (
	"context"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

// benchBlockID mirrors the rpc/v10.BlockID shape used as a representative
// struct param across most Starknet handlers. Defined locally to avoid
// importing rpc/* (would cycle back to jsonrpc).
type benchBlockID struct {
	Number uint64 `json:"block_number,omitempty"`
	Hash   string `json:"block_hash,omitempty"`
	Tag    string `json:"tag,omitempty"`
}

type benchValidatedParam struct {
	A int `json:"a" validate:"min=1"`
}

type benchSmallStruct struct {
	Index int    `json:"index"`
	Tag   string `json:"tag"`
}

func benchServer(b *testing.B, withValidator bool, methods ...jsonrpc.Method) *jsonrpc.Server {
	b.Helper()
	s := jsonrpc.NewServer(1, log.NewNopZapLogger())
	if withValidator {
		s = s.WithValidator(validator.New())
	}
	require.NoError(b, s.RegisterMethods(methods...))
	return s
}

func runBench(b *testing.B, server *jsonrpc.Server, request string) {
	b.Helper()
	b.ReportAllocs()
	for b.Loop() {
		_, _, err := server.HandleReader(b.Context(), strings.NewReader(request))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHandle_HotPath exercises the request pipeline across the axes
// that matter for the sonic migration: param decoding (positional vs
// named, scalar vs struct, optional present vs missing), validator path,
// error paths, batch fan-out, and a response-marshal-heavy axis. Sub-bench
// names are kept short so `benchstat` output is readable.
func BenchmarkHandle_HotPath(b *testing.B) {
	b.Run("no_params", func(b *testing.B) {
		s := benchServer(b, false, jsonrpc.Method{
			Name:    "noargs",
			Handler: func() (int, *jsonrpc.Error) { return 1, nil },
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"noargs"}`)
	})

	b.Run("no_params_ctx", func(b *testing.B) {
		s := benchServer(b, false, jsonrpc.Method{
			Name:    "noargs",
			Handler: func(ctx context.Context) (int, *jsonrpc.Error) { return 1, nil },
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"noargs"}`)
	})

	b.Run("one_scalar_pos", func(b *testing.B) {
		s := benchServer(b, false, jsonrpc.Method{
			Name:    "scalar",
			Params:  []jsonrpc.Parameter{{Name: "n"}},
			Handler: func(n *int) (int, *jsonrpc.Error) { return *n, nil },
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"scalar","params":[42]}`)
	})

	b.Run("one_struct_pos", func(b *testing.B) {
		s := benchServer(b, false, jsonrpc.Method{
			Name:    "struct1",
			Params:  []jsonrpc.Parameter{{Name: "block"}},
			Handler: func(blk *benchBlockID) (int, *jsonrpc.Error) { return int(blk.Number), nil },
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"struct1","params":[{"block_number":12345}]}`)
	})

	b.Run("three_pos_mixed", func(b *testing.B) {
		s := benchServer(b, false, jsonrpc.Method{
			Name:   "mixed",
			Params: []jsonrpc.Parameter{{Name: "n"}, {Name: "s"}, {Name: "block"}},
			Handler: func(n *int, str string, blk *benchBlockID) (int, *jsonrpc.Error) {
				return *n, nil
			},
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"mixed","params":[1,"hello",{"block_number":99}]}`)
	})

	b.Run("three_named_required", func(b *testing.B) {
		s := benchServer(b, false, jsonrpc.Method{
			Name:   "named",
			Params: []jsonrpc.Parameter{{Name: "n"}, {Name: "s"}, {Name: "block"}},
			Handler: func(n *int, str string, blk *benchBlockID) (int, *jsonrpc.Error) {
				return *n, nil
			},
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"named","params":{"n":1,"s":"hello","block":{"block_number":99}}}`)
	})

	b.Run("named_opt_missing", func(b *testing.B) {
		s := benchServer(b, false, jsonrpc.Method{
			Name: "namedopt",
			Params: []jsonrpc.Parameter{
				{Name: "n"},
				{Name: "s", Optional: true},
				{Name: "block", Optional: true},
			},
			Handler: func(n *int, str string, blk *benchBlockID) (int, *jsonrpc.Error) {
				return *n, nil
			},
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"namedopt","params":{"n":1}}`)
	})

	b.Run("named_opt_present", func(b *testing.B) {
		s := benchServer(b, false, jsonrpc.Method{
			Name: "namedopt",
			Params: []jsonrpc.Parameter{
				{Name: "n"},
				{Name: "s", Optional: true},
				{Name: "block", Optional: true},
			},
			Handler: func(n *int, str string, blk *benchBlockID) (int, *jsonrpc.Error) {
				return *n, nil
			},
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"namedopt","params":{"n":1,"s":"hi","block":{"block_number":99}}}`)
	})

	b.Run("validator_pass", func(b *testing.B) {
		s := benchServer(b, true, jsonrpc.Method{
			Name:    "vparam",
			Params:  []jsonrpc.Parameter{{Name: "p"}},
			Handler: func(p *benchValidatedParam) (int, *jsonrpc.Error) { return p.A, nil },
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"vparam","params":[{"a":5}]}`)
	})

	b.Run("validator_fail", func(b *testing.B) {
		s := benchServer(b, true, jsonrpc.Method{
			Name:    "vparam",
			Params:  []jsonrpc.Parameter{{Name: "p"}},
			Handler: func(p *benchValidatedParam) (int, *jsonrpc.Error) { return p.A, nil },
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"vparam","params":[{"a":0}]}`)
	})

	b.Run("type_error", func(b *testing.B) {
		s := benchServer(b, false, jsonrpc.Method{
			Name:    "scalar",
			Params:  []jsonrpc.Parameter{{Name: "n"}},
			Handler: func(n *int) (int, *jsonrpc.Error) { return *n, nil },
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"scalar","params":["not a number"]}`)
	})

	b.Run("batch_small", func(b *testing.B) {
		s := benchServer(b, false, jsonrpc.Method{
			Name:    "noargs",
			Handler: func() (int, *jsonrpc.Error) { return 1, nil },
		})
		runBench(b, s, batchRequest(5, `{"jsonrpc":"2.0","id":1,"method":"noargs"}`))
	})

	b.Run("batch_large", func(b *testing.B) {
		s := benchServer(b, false, jsonrpc.Method{
			Name:    "noargs",
			Handler: func() (int, *jsonrpc.Error) { return 1, nil },
		})
		runBench(b, s, batchRequest(50, `{"jsonrpc":"2.0","id":1,"method":"noargs"}`))
	})

	b.Run("response_100_structs", func(b *testing.B) {
		result := make([]benchSmallStruct, 100)
		for i := range result {
			result[i] = benchSmallStruct{Index: i, Tag: "tag"}
		}
		s := benchServer(b, false, jsonrpc.Method{
			Name:    "list",
			Handler: func() ([]benchSmallStruct, *jsonrpc.Error) { return result, nil },
		})
		runBench(b, s, `{"jsonrpc":"2.0","id":1,"method":"list"}`)
	})
}

func batchRequest(n int, one string) string {
	var sb strings.Builder
	sb.Grow((len(one) + 1) * n)
	sb.WriteByte('[')
	for i := range n {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(one)
	}
	sb.WriteByte(']')
	return sb.String()
}

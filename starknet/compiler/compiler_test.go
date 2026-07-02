package compiler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompileFFI(t *testing.T) {
	t.Run("zero sierra", func(t *testing.T) {
		_, err := compiler.CompileFFI(&starknet.SierraClass{})
		require.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		cl := feeder.NewTestClient(t, &networks.Integration)
		classHash := felt.NewUnsafeFromString[felt.Felt](
			"0xc6c634d10e2cc7b1db6b4403b477f05e39cb4900fd5ea0156d1721dbb6c59b",
		)

		classDef, err := cl.ClassDefinition(t.Context(), classHash)
		require.NoError(t, err)
		compiledDef, err := cl.CasmClassDefinition(t.Context(), classHash)
		require.NoError(t, err)

		expectedCompiled, err := sn2core.AdaptCompiledClass(&compiledDef)
		require.NoError(t, err)

		res, err := compiler.CompileFFI(classDef.Sierra)
		require.NoError(t, err)

		gotCompiled, err := sn2core.AdaptCompiledClass(res)
		require.NoError(t, err)
		assert.Equal(
			t,
			expectedCompiled.Hash(core.HashVersionV1),
			gotCompiled.Hash(core.HashVersionV1),
		)
	})

	t.Run("declare cairo2 class", func(t *testing.T) {
		// tests https://github.com/NethermindEth/juno/issues/1748
		definition := loadTestData[starknet.SierraClass](t, "declare_cairo2_definition.json")

		_, err := compiler.CompileFFI(&definition)
		require.NoError(t, err)
	})
}

func TestCompile(t *testing.T) {
	c := compiler.New(
		&compiler.Config{},
		"",
		log.NewNopZapLogger(),
	)

	t.Run("cancelled context returns error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := c.Compile(ctx, &starknet.SierraClass{})
		require.Error(t, err)
	})

	t.Run("zero timeout returns error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
		defer cancel()
		time.Sleep(time.Millisecond) // ensure timeout fires

		_, err := c.Compile(ctx, &starknet.SierraClass{})
		require.Error(t, err)
	})
}

func TestCompileCPULimit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("rlimits are only enforced on Linux")
	}

	// A fake compiler binary that stands in for `juno compile-sierra`: it
	// applies the CPU-time limit passed by the parent to itself (as the
	// real child does), then burns CPU for ~4 wall-clock seconds and,
	// only if it survives, prints a valid (empty) CASM JSON document.
	script := writeScript(t, "busy.sh",
		"#!/bin/sh\n"+
			"cpu=0\n"+
			"while [ $# -gt 0 ]; do\n"+
			"  case \"$1\" in\n"+
			"    --max-cpu-time) cpu=$2; shift 2;;\n"+
			"    *) shift;;\n"+
			"  esac\n"+
			"done\n"+
			"[ \"$cpu\" -gt 0 ] && ulimit -t \"$cpu\"\n"+
			"end=$(($(date +%s)+4))\n"+
			"while [ \"$(date +%s)\" -lt \"$end\" ]; do\n"+
			"  i=0\n"+
			"  while [ \"$i\" -lt 100000 ]; do i=$((i+1)); done\n"+
			"done\n"+
			"echo '{}'\n",
	)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	// Without a limit the same script must succeed, proving that the
	// failure below is caused by the CPU time limit rather than by the
	// environment.
	unlimited := compiler.New(
		&compiler.Config{},
		script,
		log.NewNopZapLogger(),
	)
	_, err := unlimited.Compile(ctx, &starknet.SierraClass{})
	require.NoError(t, err)

	limited := compiler.New(
		&compiler.Config{MaxCPUTime: 1},
		script,
		log.NewNopZapLogger(),
	)
	_, err = limited.Compile(ctx, &starknet.SierraClass{})
	require.ErrorContains(t, err, "exceeded max CPU time")
	require.NotErrorIs(t, err, context.DeadlineExceeded)
}

func TestCompileMemoryLimit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("rlimits are only enforced on Linux")
	}

	// A fake compiler binary that stands in for `juno compile-sierra`: it
	// applies the memory limit passed by the parent to itself (as the
	// real child does), then allocates a 256 MB buffer and, only if that
	// succeeds, prints a valid (empty) CASM JSON document. ulimit -v is
	// in KB, while the parent passes RLIMIT_AS in bytes.
	script := writeScript(t, "hungry.sh",
		"#!/bin/sh\n"+
			"mem=0\n"+
			"while [ $# -gt 0 ]; do\n"+
			"  case \"$1\" in\n"+
			"    --max-memory) mem=$2; shift 2;;\n"+
			"    *) shift;;\n"+
			"  esac\n"+
			"done\n"+
			"[ \"$mem\" -gt 0 ] && ulimit -v $((mem/1024))\n"+
			"dd if=/dev/zero of=/dev/null bs=$((256*1024*1024)) count=1 || exit 1\n"+
			"echo '{}'\n",
	)

	ctx, cancel := context.WithTimeout(t.Context(), 31*time.Second)
	defer cancel()

	// Without a limit the same script must succeed, proving that the
	// failure below is caused by the memory limit rather than by the
	// environment (e.g. a dd variant rejecting the block size).
	unlimited := compiler.New(
		&compiler.Config{},
		script,
		log.NewNopZapLogger(),
	)
	_, err := unlimited.Compile(ctx, &starknet.SierraClass{})
	require.NoError(t, err)

	limited := compiler.New(
		&compiler.Config{MaxMemory: 64 << 20},
		script,
		log.NewNopZapLogger(),
	)
	_, err = limited.Compile(ctx, &starknet.SierraClass{})
	require.Error(t, err)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
}

func writeScript(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o700))
	return path
}

// loadTestData loads json file located relative to a test package and unmarshal it to provided type
func loadTestData[T any](t *testing.T, filename string) T {
	t.Helper()

	file := fmt.Sprintf("../testdata/%s", filename)
	buff, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", file, err)
	}

	var v T
	err = json.Unmarshal(buff, &v)
	if err != nil {
		t.Fatalf("Failed to unmarshal json: %v", err)
	}

	// todo check for zero value
	return v
}

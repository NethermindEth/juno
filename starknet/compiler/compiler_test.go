package compiler_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

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

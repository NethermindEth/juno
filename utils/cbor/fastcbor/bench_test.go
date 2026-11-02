package fastcbor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

// BenchmarkMainnetRecords reads the records of the corpus into fresh destinations with the fast
// package and with fxamacker.
func BenchmarkMainnetRecords(b *testing.B) {
	files, _ := filepath.Glob(filepath.Join(corpus, "*.bin"))
	if len(files) == 0 {
		b.Skip("no corpus")
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".bin")
		data, err := os.ReadFile(file)
		require.NoError(b, err)
		switch {
		case strings.HasPrefix(name, "receipt"):
			benchmark[*core.TransactionReceipt](b, name, data)
		case strings.HasPrefix(name, "header"):
			benchmark[core.Header](b, name, data)
		case strings.HasPrefix(name, "state"):
			benchmark[core.StateUpdate](b, name, data)
		case strings.HasPrefix(name, "transaction"):
			benchmark[core.Transaction](b, name, data)
		case strings.HasPrefix(name, "class"):
			// A DeclaredClassDefinition reads its class with the fastcbor package even when fxamacker
			// reads the record, so the class is measured on its own, after the 8 bytes of the block.
			benchmark[core.ClassDefinition](b, name, data[8:])
		}
	}
}

func benchmark[T any](b *testing.B, name string, data []byte) {
	b.Run(name+"/fast", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var dst T
			if err := fastcbor.Unmarshal(data, &dst); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run(name+"/fxamacker", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var dst T
			if err := fxamacker.Unmarshal(data, &dst); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkMainnetRecordsParallel reads a few records from all the procs at once, to see whether
// the fastcbor package scales with them as fxamacker does. Run it with -cpu 1,2,4.
func BenchmarkMainnetRecordsParallel(b *testing.B) {
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(corpus, name+".bin"))
		if err != nil {
			b.Skip("no corpus")
		}
		return data
	}
	parallel[*core.TransactionReceipt](b, "receipt-p50", read("receipt-p50"))
	parallel[core.Header](b, "header-p50", read("header-p50"))
	parallel[core.StateUpdate](b, "state-p50", read("state-p50"))
	parallel[core.Transaction](b, "transactionInvoke-p50", read("transactionInvoke-p50"))
	parallel[core.ClassDefinition](b, "class-p50", read("class-p50")[8:])
}

func parallel[T any](b *testing.B, name string, data []byte) {
	for _, mode := range []struct {
		name string
		read func([]byte, *T) error
	}{
		{"fast", func(d []byte, v *T) error { return fastcbor.Unmarshal(d, v) }},
		{"fxamacker", func(d []byte, v *T) error { return fxamacker.Unmarshal(d, v) }},
	} {
		b.Run(name+"/"+mode.name, func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					var dst T
					if err := mode.read(data, &dst); err != nil {
						b.Error(err)
						return
					}
				}
			})
		})
	}
}

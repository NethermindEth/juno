package fast_test

import (
	"strings"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/stretchr/testify/require"
)

func TestFieldsOrder(t *testing.T) {
	long := strings.Repeat("x", 30)
	fields := fast.NewFields("ccc", long, "bb", "a", "B0")
	require.Equal(t, []string{"a", "B0", "bb", "ccc", long}, fields.Order())
}

func TestNewFieldsRejects(t *testing.T) {
	require.Panics(t, func() { fast.NewFields("Fee", "fee") })
	require.Panics(t, func() { fast.NewFields("fé") })
	require.Panics(t, func() { fast.NewFields(strings.Repeat("x", 256)) })
}

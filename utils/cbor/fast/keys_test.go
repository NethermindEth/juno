package fast_test

import (
	"strings"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/stretchr/testify/require"
)

func TestNewFieldsRejects(t *testing.T) {
	require.Panics(t, func() { fast.NewFields("Fee", "fee") })
	require.Panics(t, func() { fast.NewFields("fé") })
	require.Panics(t, func() { fast.NewFields(strings.Repeat("x", 256)) })
}

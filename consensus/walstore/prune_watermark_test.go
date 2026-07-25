package walstore_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/walstore"
	"github.com/stretchr/testify/require"
)

const testPruneWatermarkHeight = types.Height(1)

func TestPruneWatermarkRoundTrips(t *testing.T) {
	walDir := t.TempDir()

	height, err := walstore.LoadPruneWatermark(walDir)
	require.NoError(t, err)
	require.Zero(t, height)

	require.NoError(t, walstore.WritePruneWatermark(walDir, testPruneWatermarkHeight))
	height, err = walstore.LoadPruneWatermark(walDir)
	require.NoError(t, err)
	require.Equal(t, testPruneWatermarkHeight, height)
}

func TestLoadPruneWatermarkRejectsCorruptFile(t *testing.T) {
	walDir := t.TempDir()
	require.NoError(t, walstore.WritePruneWatermark(walDir, testPruneWatermarkHeight))
	validWatermark, err := os.ReadFile(filepath.Join(walDir, "prune-watermark"))
	require.NoError(t, err)

	wrongHeader := make([]byte, len(validWatermark))
	copy(wrongHeader, "wrong-header")

	tests := map[string][]byte{
		"wrong size": []byte("short"),
		"bad header": wrongHeader,
	}

	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			walDir := t.TempDir()
			path := filepath.Join(walDir, "prune-watermark")
			require.NoError(t, os.WriteFile(path, contents, 0o644))

			_, err := walstore.LoadPruneWatermark(walDir)
			require.Error(t, err)
		})
	}
}

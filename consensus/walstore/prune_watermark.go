package walstore

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NethermindEth/juno/consensus/types"
)

const (
	pruneWatermarkHeader = "juno-wal-prune-watermark-v1"
	pruneWatermarkSize   = len(pruneWatermarkHeader) + 8
)

func loadPruneWatermark(walDir string) (types.Height, error) {
	data, err := os.ReadFile(pruneWatermarkPath(walDir))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("loadPruneWatermark: read watermark: %w", err)
	}
	if len(data) != pruneWatermarkSize {
		return 0, fmt.Errorf("loadPruneWatermark: invalid watermark size %d", len(data))
	}
	if string(data[:len(pruneWatermarkHeader)]) != pruneWatermarkHeader {
		return 0, errors.New("loadPruneWatermark: invalid watermark header")
	}
	return types.Height(binary.BigEndian.Uint64(data[len(pruneWatermarkHeader):])), nil
}

func writePruneWatermark(walDir string, height types.Height) error {
	const pruneWatermarkFilePerm = 0o644

	var data [pruneWatermarkSize]byte
	copy(data[:], pruneWatermarkHeader)
	binary.BigEndian.PutUint64(data[len(pruneWatermarkHeader):], uint64(height))

	path := pruneWatermarkPath(walDir)
	tmpPath := path + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, pruneWatermarkFilePerm)
	if err != nil {
		return fmt.Errorf("writePruneWatermark: create temp watermark: %w", err)
	}

	_, writeErr := file.Write(data[:])
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		if writeErr != nil {
			writeErr = fmt.Errorf("writePruneWatermark: write temp watermark: %w", writeErr)
		}
		if closeErr != nil {
			closeErr = fmt.Errorf("writePruneWatermark: close temp watermark: %w", closeErr)
		}
		return errors.Join(writeErr, closeErr)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writePruneWatermark: replace watermark: %w", err)
	}
	if err := syncDir(walDir); err != nil {
		return fmt.Errorf("writePruneWatermark: sync watermark directory: %w", err)
	}
	return nil
}

func pruneWatermarkPath(walDir string) string {
	return filepath.Join(walDir, "prune-watermark")
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()

	if err := dir.Sync(); err != nil {
		return err
	}
	return nil
}

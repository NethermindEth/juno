package db_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	consensusdb "github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/cockroachdb/pebble/v2/record"
	pebblewal "github.com/cockroachdb/pebble/v2/wal"
	"github.com/stretchr/testify/require"
)

func TestNewTendermintWALStoreFailsOnCorruptWALBatch(t *testing.T) {
	dbPath := t.TempDir()
	walDir := consensusdb.DefaultWALDir(dbPath)
	require.NoError(t, os.MkdirAll(walDir, 0o755))
	walPath := filepath.Join(walDir, pebblewal.NumWAL(1).String()+".log")
	appendValidWALRecord(t, walPath, []byte("not-a-batch"))

	testDB, err := pebblev2.New(dbPath)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, testDB.Close())
	}()

	_, err = consensusdb.NewTendermintWALStore[starknet.Value, starknet.Hash, starknet.Address](testDB)
	require.Error(t, err)
}

func appendValidWALRecord(t *testing.T, path string, payload []byte) {
	t.Helper()

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	require.NoError(t, err)
	defer file.Close()

	_, err = file.Seek(0, io.SeekEnd)
	require.NoError(t, err)

	writer := record.NewWriter(file)
	_, err = writer.WriteRecord(payload)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.NoError(t, file.Sync())
}

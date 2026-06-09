package db_test

import (
	"os"
	"path/filepath"
	"testing"

	consensusdb "github.com/NethermindEth/juno/consensus/db"
	"github.com/stretchr/testify/require"
)

func TestWALReplayTruncatesInvalidLatestWALTail(t *testing.T) {
	expectedEntries := makeWALEntriesForHeight(1)
	walStore, testDB, dbPath := newTestTendermintWALStore(t)
	writeAndFlushEntries(t, walStore, expectedEntries)
	require.NoError(t, walStore.Close())
	require.NoError(t, testDB.Close())

	walDir := consensusdb.DefaultWALDir(dbPath)
	walPath := filepath.Join(walDir, "000001.log")

	info, err := os.Stat(walPath)
	require.NoError(t, err)
	sizeBeforeInvalidTail := info.Size()

	appendInvalidWALTail(t, walPath, []byte("partial-record-tail"))
	info, err = os.Stat(walPath)
	require.NoError(t, err)
	corruptedSize := info.Size()
	require.Greater(t, corruptedSize, sizeBeforeInvalidTail)

	walStore, testDB = openTestTendermintWALStore(t, dbPath)
	defer func() {
		require.NoError(t, walStore.Close())
		require.NoError(t, testDB.Close())
	}()

	assertLoadedEntries(t, walStore, expectedEntries)

	info, err = os.Stat(walPath)
	require.NoError(t, err)
	require.LessOrEqual(t, info.Size(), sizeBeforeInvalidTail)
}

func appendInvalidWALTail(t *testing.T, path string, payload []byte) {
	t.Helper()

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	require.NoError(t, err)
	defer file.Close()

	_, err = file.Write(payload)
	require.NoError(t, err)
	require.NoError(t, file.Sync())
}

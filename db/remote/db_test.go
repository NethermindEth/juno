package remote

import (
	"net"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	junogrpc "github.com/NethermindEth/juno/grpc"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestRemote(t *testing.T) {
	memDB := memory.New()
	require.NoError(t, memDB.Update(func(txn db.IndexedBatch) error {
		for i := byte(0); i < 3; i++ {
			if err := txn.Put([]byte{i}, []byte{i}); err != nil {
				return err
			}
		}
		return nil
	}))

	grpcHandler := junogrpc.New(memDB, "0.0.0")
	grpcSrv := grpc.NewServer()
	gen.RegisterKVServer(grpcSrv, grpcHandler)

	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		require.NoError(t, grpcSrv.Serve(l))
	}()

	remoteDB, err := New(
		l.Addr().String(),
		t.Context(),
		log.NewNopZapLogger(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Run("Get", func(t *testing.T) {
		snap := remoteDB.NewSnapshot()
		defer snap.Close()

		for i := byte(0); i < 3; i++ {
			require.NoError(t, snap.Get([]byte{i}, func(data []byte) error {
				assert.Equal(t, []byte{i}, data)
				return nil
			}))

			missing := snap.Get([]byte{0xDE, 0xAD}, func(b []byte) error { return nil })
			assert.Equal(t, db.ErrKeyNotFound, missing)
		}
	})

	t.Run("iterate", func(t *testing.T) {
		snap := remoteDB.NewSnapshot()
		defer snap.Close()

		it, err := snap.NewIterator(nil, false)
		require.NoError(t, err)
		defer it.Close()

		foundKeys := byte(0)
		for valid := it.Next(); valid; valid = it.Next() {
			assert.Equal(t, []byte{foundKeys}, it.Key())
			v, err := it.Value()
			require.NoError(t, err)
			assert.Equal(t, v, []byte{foundKeys})
			foundKeys++
		}
		assert.Equal(t, foundKeys, byte(3))
	})

	t.Run("first", func(t *testing.T) {
		snap := remoteDB.NewSnapshot()
		defer snap.Close()

		it, err := snap.NewIterator(nil, false)
		require.NoError(t, err)
		defer it.Close()

		require.True(t, it.First())
		assert.Equal(t, []byte{0}, it.Key())
		v, err := it.Value()
		require.NoError(t, err)
		assert.Equal(t, []byte{0}, v)
	})

	t.Run("seek", func(t *testing.T) {
		snap := remoteDB.NewSnapshot()
		defer snap.Close()

		it, err := snap.NewIterator(nil, false)
		require.NoError(t, err)
		defer it.Close()

		assert.True(t, it.Seek([]byte{1}))
		assert.Equal(t, it.Key(), []byte{1})
		v, err := it.Value()
		require.NoError(t, err)
		assert.Equal(t, v, []byte{1})
	})

	t.Run("write", func(t *testing.T) {
		err := remoteDB.Update(func(txn db.IndexedBatch) error {
			assert.EqualError(t, txn.Delete(nil), "read only DB")
			assert.EqualError(t, txn.Put(nil, nil), "read only DB")
			return nil
		})
		assert.EqualError(t, err, "read only DB")
	})
	grpcSrv.GracefulStop()
}

// TestRemoteIteratorBounds guards against the bounds being dropped on the wire:
// a key sorting before the prefix and one sorting after it must not surface.
func TestRemoteIteratorBounds(t *testing.T) {
	memDB := memory.New()
	require.NoError(t, memDB.Update(func(txn db.IndexedBatch) error {
		keys := [][]byte{
			{0x00, 0xFF},
			{0x01, 0x00},
			{0x01, 0x01},
			{0x01, 0x02},
			{0x02, 0x00},
		}
		for _, k := range keys {
			if err := txn.Put(k, k); err != nil {
				return err
			}
		}
		return nil
	}))

	grpcHandler := junogrpc.New(memDB, "0.0.0")
	grpcSrv := grpc.NewServer()
	gen.RegisterKVServer(grpcSrv, grpcHandler)

	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		require.NoError(t, grpcSrv.Serve(l))
	}()
	defer grpcSrv.GracefulStop()

	remoteDB, err := New(
		l.Addr().String(),
		t.Context(),
		log.NewNopZapLogger(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	// Top-level NewIterator, not a snapshot's, so this also exercises ownedIterator.
	it, err := remoteDB.NewIterator([]byte{0x01}, true)
	require.NoError(t, err)
	defer it.Close()

	var found [][]byte
	for valid := it.First(); valid; valid = it.Next() {
		found = append(found, slices.Clone(it.Key()))
	}
	assert.Equal(t, [][]byte{{0x01, 0x00}, {0x01, 0x01}, {0x01, 0x02}}, found)
}

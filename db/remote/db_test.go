package remote

import (
	"bytes"
	"errors"
	"net"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	junogrpc "github.com/NethermindEth/juno/grpc"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils"
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

	remoteDB, err := New(l.Addr().String(), t.Context(), utils.NewNopZapLogger(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	t.Run("Get", func(t *testing.T) {
		assert.NoError(t, remoteDB.View(func(txn db.Snapshot) error {
			for i := byte(0); i < 3; i++ {
				err := txn.Get([]byte{i}, func(data []byte) error {
					if !bytes.Equal(data, []byte{i}) {
						return errors.New("wrong value")
					}
					return nil
				})
				if err != nil {
					return err
				}

				assert.Equal(t, db.ErrKeyNotFound, txn.Get([]byte{0xDE, 0xAD}, func(b []byte) error { return nil }))
			}
			return nil
		}))
	})

	t.Run("iterate", func(t *testing.T) {
		err := remoteDB.View(func(txn db.Snapshot) error {
			it, err := txn.NewIterator(nil, false)
			if err != nil {
				return err
			}
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
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("seek", func(t *testing.T) {
		err := remoteDB.View(func(txn db.Snapshot) error {
			it, err := txn.NewIterator(nil, false)
			if err != nil {
				return err
			}
			defer it.Close()

			assert.True(t, it.Seek([]byte{1}))
			assert.Equal(t, it.Key(), []byte{1})
			v, err := it.Value()
			require.NoError(t, err)
			assert.Equal(t, v, []byte{1})

			return nil
		})
		assert.NoError(t, err)
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

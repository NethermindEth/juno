package remote_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/db/remote"
	junogrpc "github.com/NethermindEth/juno/grpc"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestRemote(t *testing.T) {
	memDB := pebble.NewMemTest(t)
	require.NoError(t, memDB.Update(func(txn db.Transaction) error {
		for i := byte(0); i < 3; i++ {
			if err := txn.Set([]byte{i}, []byte{i}); err != nil {
				return err
			}
		}
		return nil
	}))

	grpcHandler := junogrpc.New(memDB, "0.0.0")
	grpcSrv := grpc.NewServer()
	gen.RegisterKVServer(grpcSrv, grpcHandler)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		require.NoError(t, grpcSrv.Serve(l))
	}()

	remoteDB, err := remote.New(l.Addr().String(), context.Background(), utils.NewNopZapLogger(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	t.Run("Get", func(t *testing.T) {
		assert.NoError(t, remoteDB.View(func(txn db.Transaction) error {
			for i := byte(0); i < 3; i++ {
				if err := txn.Get([]byte{i}, func(b []byte) error {
					if !bytes.Equal(b, []byte{i}) {
						return errors.New("wrong value")
					}
					return nil
				}); err != nil {
					return err
				}
			}

			assert.Equal(t, db.ErrKeyNotFound, txn.Get([]byte{0xDE, 0xAD}, func(b []byte) error { return nil }))
			return nil
		}))
	})

	t.Run("iterate", func(t *testing.T) {
		err := remoteDB.View(func(txn db.Transaction) error {
			it, err := txn.NewIterator()
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
		err := remoteDB.View(func(txn db.Transaction) error {
			it, err := txn.NewIterator()
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
		err := remoteDB.Update(func(txn db.Transaction) error {
			assert.EqualError(t, txn.Delete(nil), "read only DB")
			assert.EqualError(t, txn.Set(nil, nil), "read only DB")
			return nil
		})
		assert.EqualError(t, err, "read only DB")
	})
	grpcSrv.GracefulStop()
}

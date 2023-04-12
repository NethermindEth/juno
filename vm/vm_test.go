package vm

import (
	"context"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/encoder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV0Call(t *testing.T) {
	testDB := pebble.NewMemTest()
	txn := testDB.NewTransaction(true)
	client, closer := feeder.NewTestClient(utils.MAINNET)
	gw := adaptfeeder.New(client)
	t.Cleanup(func() {
		closer()
		require.NoError(t, txn.Discard())
		require.NoError(t, testDB.Close())
	})

	contractAddr := utils.HexToFelt(t, "0xDEADBEEF")
	// https://voyager.online/class/0x03297a93c52357144b7da71296d7e8231c3e0959f0a1d37222204f2f7712010e
	classHash := utils.HexToFelt(t, "0x3297a93c52357144b7da71296d7e8231c3e0959f0a1d37222204f2f7712010e")
	simpleClass, err := gw.Class(context.Background(), classHash)
	require.NoError(t, err)

	require.NoError(t, encoder.RegisterType(reflect.TypeOf(core.Cairo0Class{})))
	require.NoError(t, encoder.RegisterType(reflect.TypeOf(core.Cairo1Class{})))

	testState := core.NewState(txn)
	require.NoError(t, testState.Update(0, &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: utils.HexToFelt(t, "0x3d452fbb3c3a32fe85b1a3fbbcdec316d5fc940cefc028ee808ad25a15991c8"),
		StateDiff: &core.StateDiff{
			DeployedContracts: []core.DeployedContract{
				{
					Address:   contractAddr,
					ClassHash: classHash,
				},
			},
		},
	}, map[felt.Felt]core.Class{
		*classHash: simpleClass,
	}))

	entryPoint := utils.HexToFelt(t, "0x39e11d48192e4333233c7eb19d10ad67c362bb28580c604d67884c85da39695")
	ret, err := Call(contractAddr, entryPoint, nil, 0, 0, testState, utils.MAINNET)
	require.NoError(t, err)
	assert.Equal(t, []*felt.Felt{&felt.Zero}, ret)

	require.NoError(t, testState.Update(1, &core.StateUpdate{
		OldRoot: utils.HexToFelt(t, "0x3d452fbb3c3a32fe85b1a3fbbcdec316d5fc940cefc028ee808ad25a15991c8"),
		NewRoot: utils.HexToFelt(t, "0x4a948783e8786ba9d8edaf42de972213bd2deb1b50c49e36647f1fef844890f"),
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt][]core.StorageDiff{
				*contractAddr: {
					core.StorageDiff{
						Key:   utils.HexToFelt(t, "0x206f38f7e4f15e87567361213c28f235cccdaa1d7fd34c9db1dfe9489c6a091"),
						Value: new(felt.Felt).SetUint64(1337),
					},
				},
			},
		},
	}, map[felt.Felt]core.Class{
		*classHash: simpleClass,
	}))

	ret, err = Call(contractAddr, entryPoint, nil, 1, 0, testState, utils.MAINNET)
	require.NoError(t, err)
	assert.Equal(t, []*felt.Felt{new(felt.Felt).SetUint64(1337)}, ret)
}

func TestV1Call(t *testing.T) {
	testDB := pebble.NewMemTest()
	txn := testDB.NewTransaction(true)
	client, closer := feeder.NewTestClient(utils.GOERLI)
	gw := adaptfeeder.New(client)
	t.Cleanup(func() {
		closer()
		require.NoError(t, txn.Discard())
		require.NoError(t, testDB.Close())
	})

	contractAddr := utils.HexToFelt(t, "0xDEADBEEF")
	// https://goerli.voyager.online/class/0x01338d85d3e579f6944ba06c005238d145920afeb32f94e3a1e234d21e1e9292
	classHash := utils.HexToFelt(t, "0x1338d85d3e579f6944ba06c005238d145920afeb32f94e3a1e234d21e1e9292")
	simpleClass, err := gw.Class(context.Background(), classHash)
	require.NoError(t, err)

	testState := core.NewState(txn)
	require.NoError(t, testState.Update(0, &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: utils.HexToFelt(t, "0x2650cef46c190ec6bb7dc21a5a36781132e7c883b27175e625031149d4f1a84"),
		StateDiff: &core.StateDiff{
			DeployedContracts: []core.DeployedContract{
				{
					Address:   contractAddr,
					ClassHash: classHash,
				},
			},
		},
	}, map[felt.Felt]core.Class{
		*classHash: simpleClass,
	}))

	// test_storage_read
	entryPoint := utils.HexToFelt(t, "0x5df99ae77df976b4f0e5cf28c7dcfe09bd6e81aab787b19ac0c08e03d928cf")
	storageLocation := utils.HexToFelt(t, "0x44")
	ret, err := Call(contractAddr, entryPoint, []*felt.Felt{
		storageLocation,
	}, 0, 0, testState, utils.GOERLI)
	require.NoError(t, err)
	assert.Equal(t, []*felt.Felt{&felt.Zero}, ret)

	require.NoError(t, testState.Update(1, &core.StateUpdate{
		OldRoot: utils.HexToFelt(t, "0x2650cef46c190ec6bb7dc21a5a36781132e7c883b27175e625031149d4f1a84"),
		NewRoot: utils.HexToFelt(t, "0x7a9da0a7471a8d5118d3eefb8c26a6acbe204eb1eaa934606f4757a595fe552"),
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt][]core.StorageDiff{
				*contractAddr: {
					core.StorageDiff{
						Key:   storageLocation,
						Value: new(felt.Felt).SetUint64(37),
					},
				},
			},
		},
	}, map[felt.Felt]core.Class{
		*classHash: simpleClass,
	}))

	ret, err = Call(contractAddr, entryPoint, []*felt.Felt{
		storageLocation,
	}, 1, 0, testState, utils.GOERLI)
	require.NoError(t, err)
	assert.Equal(t, []*felt.Felt{new(felt.Felt).SetUint64(37)}, ret)
}

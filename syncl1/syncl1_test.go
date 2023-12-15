package syncl1_test

import (
	"bytes"
	"context"
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/l1data"
	"github.com/NethermindEth/juno/mocks"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/syncl1"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//go:embed testdata
var testdata embed.FS

func getEncodedDiffs(t *testing.T) [][]*big.Int {
	t.Helper()
	encodedDiffs := make([][]*big.Int, 3)
	for i := 0; i < 3; i++ {
		f, err := testdata.Open(fmt.Sprintf("testdata/%d.json", i))
		require.NoError(t, err)
		encodedDiffBytes, err := io.ReadAll(f)
		require.NoError(t, err)
		var encodedDiffStrings []string
		require.NoError(t, json.Unmarshal(encodedDiffBytes, &encodedDiffStrings))
		encodedDiff := make([]*big.Int, len(encodedDiffStrings))
		for i, x := range encodedDiffStrings {
			y, ok := new(big.Int).SetString(x, felt.Base16)
			require.True(t, ok)
			encodedDiff[i] = y
		}
		encodedDiffs[i] = encodedDiff
		require.NoError(t, f.Close())
	}
	return encodedDiffs
}

func TestSyncL1(t *testing.T) {
	encodedDiffs := getEncodedDiffs(t)
	ctrl := gomock.NewController(t)
	network := utils.Mainnet
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)
	expectedDB := pebble.NewMemTest(t)

	ctx := context.Background()
	height := uint64(3)
	log := utils.NewNopZapLogger()
	testDB := pebble.NewMemTest(t)
	for i := uint64(0); i < height; i++ {
		// Update expected DB.
		su, err := gw.StateUpdate(ctx, i)
		require.NoError(t, err)
		require.NoError(t, expectedDB.Update(func(txn db.Transaction) error {
			require.NoError(t, core.NewState(txn).Update(i, su, nil))
			return nil
		}))

		updateFetcher := mocks.NewMockStateUpdateLogFetcher(ctrl)
		updateFetcher.EXPECT().
			StateUpdateLogs(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*l1data.LogStateUpdate{
				{
					GlobalRoot:  su.NewRoot.BigInt(new(big.Int)),
					BlockNumber: new(big.Int).SetUint64(i),
					Raw: types.Log{
						BlockNumber: i,
						TxIndex:     uint(i),
					},
				},
			}, nil).
			Times(1)
		diffFetcher := mocks.NewMockStateDiffFetcher(ctrl)
		diffFetcher.EXPECT().
			StateDiff(gomock.Any(), uint(i), i).
			Return(encodedDiffs[i], nil).
			Times(1)
		s, err := syncl1.New(testDB, updateFetcher, diffFetcher, syncl1.MainnetConfig, i, log)
		require.NoError(t, err)
		require.NoError(t, s.Run(ctx))

		// Ensure test DB has correct metadata.
		gotL1Head := new(core.L1Head)
		var gotEthHeight uint64
		require.NoError(t, testDB.View(func(txn db.Transaction) error {
			require.NoError(t, txn.Get(db.L1Height.Key(), func(data []byte) error {
				return encoder.Unmarshal(data, gotL1Head)
			}))
			require.NoError(t, txn.Get(db.EthereumHeight.Key(), func(data []byte) error {
				gotEthHeight = binary.BigEndian.Uint64(data)
				return nil
			}))
			return nil
		}))
		require.Equal(t, &core.L1Head{
			BlockNumber: i,
			StateRoot:   su.NewRoot,
		}, gotL1Head)
		require.Equal(t, i, gotEthHeight)

		// Ensure test DB state tries match expected DB.
		require.NoError(t, testDB.View(func(txn db.Transaction) error {
			require.NoError(t, expectedDB.View(func(expectedTxn db.Transaction) error {
				testIt, err := txn.NewIterator()
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, testIt.Close())
				})
				expectedIt, err := expectedTxn.NewIterator()
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, expectedIt.Close())
				})
				bucketsMatch(t, db.StateTrie.Key(), expectedIt, testIt)
				bucketsMatch(t, db.ClassesTrie.Key(), expectedIt, testIt)
				return nil
			}))
			return nil
		}))
	}
}

func bucketsMatch(t *testing.T, prefix []byte, it1, it2 db.Iterator) {
	bucketPrefix := db.StateTrie.Key()
	require.True(t, it1.Seek(prefix))
	require.True(t, it2.Seek(prefix))
	for it1.Valid() {
		key := it1.Key()
		if !bytes.HasPrefix(key, bucketPrefix) {
			break
		}

		require.Equal(t, key, it2.Key())
		value1, err := it1.Value()
		require.NoError(t, err)
		value2, err := it2.Value()
		require.NoError(t, err)
		require.Equal(t, value1, value2)

		it1.Next()
		it2.Next()
		require.True(t, it1.Valid() == it2.Valid())
	}
}

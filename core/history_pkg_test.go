package core

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistory(t *testing.T) {
	testDB := memory.New()
	txn := testDB.NewSnapshotBatch()

	history := &history{txn: txn}
	contractAddress := new(felt.Felt).SetUint64(123)

	for desc, test := range map[string]struct {
		logger  func(location, oldValue *felt.Felt, height uint64) error
		getter  func(location *felt.Felt, height uint64) (felt.Felt, error)
		deleter func(location *felt.Felt, height uint64) error
	}{
		"contract storage": {
			logger: func(location, oldValue *felt.Felt, height uint64) error {
				return history.LogContractStorage(contractAddress, location, oldValue, height)
			},
			getter: func(location *felt.Felt, height uint64) (felt.Felt, error) {
				return history.ContractStorageAt(contractAddress, location, height)
			},
			deleter: func(location *felt.Felt, height uint64) error {
				return history.DeleteContractStorageLog(contractAddress, location, height)
			},
		},
		"contract nonce": {
			logger:  history.LogContractNonce,
			getter:  history.ContractNonceAt,
			deleter: history.DeleteContractNonceLog,
		},
		"contract class hash": {
			logger:  history.LogContractClassHash,
			getter:  history.ContractClassHashAt,
			deleter: history.DeleteContractClassHashLog,
		},
	} {
		location := new(felt.Felt).SetUint64(456)

		t.Run(desc, func(t *testing.T) {
			t.Run("no history", func(t *testing.T) {
				_, err := test.getter(location, 1)
				assert.ErrorIs(t, err, ErrCheckHeadState)
			})

			value := new(felt.Felt).SetUint64(789)

			t.Run("log value changed at height 5 and 10", func(t *testing.T) {
				assert.NoError(t, test.logger(location, &felt.Zero, 5))
				assert.NoError(t, test.logger(location, value, 10))
			})

			t.Run("get value before height 5", func(t *testing.T) {
				oldValue, err := test.getter(location, 1)
				require.NoError(t, err)
				assert.Equal(t, felt.Zero, oldValue)
			})

			t.Run("get value between height 5-10 ", func(t *testing.T) {
				oldValue, err := test.getter(location, 7)
				require.NoError(t, err)
				assert.Equal(t, value, &oldValue)
			})

			t.Run("get value on height that change happened ", func(t *testing.T) {
				oldValue, err := test.getter(location, 5)
				require.NoError(t, err)
				assert.Equal(t, value, &oldValue)

				_, err = test.getter(location, 10)
				assert.ErrorIs(t, err, ErrCheckHeadState)
			})

			t.Run("get value after height 10 ", func(t *testing.T) {
				_, err := test.getter(location, 13)
				assert.ErrorIs(t, err, ErrCheckHeadState)
			})

			t.Run("get a random location ", func(t *testing.T) {
				_, err := test.getter(new(felt.Felt).SetUint64(37), 13)
				assert.ErrorIs(t, err, ErrCheckHeadState)
			})

			require.NoError(t, test.deleter(location, 10))

			t.Run("get after delete", func(t *testing.T) {
				_, err := test.getter(location, 7)
				assert.ErrorIs(t, err, ErrCheckHeadState)
			})
		})
	}
}

package core_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hexToFelt(t *testing.T, hex string) *felt.Felt {
	f, err := new(felt.Felt).SetString(hex)
	require.NoError(t, err)
	return f
}

func TestClassHash(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.GOERLI)
	defer closeFn()
	gw := adaptfeeder.New(client)

	tests := []struct {
		classHash string
	}{
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8
			classHash: "0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
		},
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3
			classHash: "0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3",
		},
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118
			classHash: "0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118",
		},
	}

	for _, tt := range tests {
		t.Run("ClassHash", func(t *testing.T) {
			hash := hexToFelt(t, tt.classHash)
			class, err := gw.Class(context.Background(), hash)
			require.NoError(t, err)
			got := class.Hash()
			if !hash.Equal(got) {
				t.Errorf("wrong hash: got %s, want %s", got.String(), hash.String())
			}
		})
	}
}

func TestContractAddress(t *testing.T) {
	tests := []struct {
		callerAddress       *felt.Felt
		classHash           *felt.Felt
		salt                *felt.Felt
		constructorCalldata []*felt.Felt
		want                *felt.Felt
	}{
		{
			// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0
			callerAddress: hexToFelt(t, "0x0000000000000000000000000000000000000000"),
			classHash: hexToFelt(
				t, "0x46f844ea1a3b3668f81d38b5c1bd55e816e0373802aefe732138628f0133486"),
			salt: hexToFelt(
				t, "0x74dc2fe193daf1abd8241b63329c1123214842b96ad7fd003d25512598a956b"),
			constructorCalldata: []*felt.Felt{
				hexToFelt(t, "0x6d706cfbac9b8262d601c38251c5fbe0497c3a96cc91a92b08d91b61d9e70c4"),
				hexToFelt(t, "0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463"),
				hexToFelt(t, "0x2"),
				hexToFelt(t, "0x6658165b4984816ab189568637bedec5aa0a18305909c7f5726e4a16e3afef6"),
				hexToFelt(t, "0x6b648b36b074a91eee55730f5f5e075ec19c0a8f9ffb0903cefeee93b6ff328"),
			},
			want: hexToFelt(t, "0x3ec215c6c9028ff671b46a2a9814970ea23ed3c4bcc3838c6d1dcbf395263c3"),
		},
	}

	for _, tt := range tests {
		t.Run("Address", func(t *testing.T) {
			address := core.ContractAddress(tt.callerAddress, tt.classHash, tt.salt, tt.constructorCalldata)
			if !address.Equal(tt.want) {
				t.Errorf("wrong address: got %s, want %s", address.String(), tt.want.String())
			}
		})
	}
}

func TestContract(t *testing.T) {
	testDb := pebble.NewMemTest()
	defer func() {
		require.NoError(t, testDb.Close())
	}()

	txn := testDb.NewTransaction(true)
	addr := new(felt.Felt).SetUint64(44)
	class := new(felt.Felt).SetUint64(37)

	contract := core.NewContract(addr, txn)
	assert.NoError(t, contract.Deploy(class))
	assert.Error(t, contract.Deploy(class))

	assert.Equal(t, addr, contract.Address)
	got, err := contract.ClassHash()
	assert.NoError(t, err)
	assert.Equal(t, class, got)
	got, err = contract.Nonce()
	assert.NoError(t, err)
	assert.Equal(t, new(felt.Felt), got)

	storage, err := contract.Storage()

	_, err = storage.Put(addr, class)
	assert.NoError(t, err)
	root, err := storage.Root()
	assert.NoError(t, err)

	contract.UpdateStorage([]core.StorageDiff{{addr, class}})

	sRoot, err := contract.StorageRoot()
	assert.NoError(t, err)
	assert.Equal(t, root, sRoot)

	assert.NoError(t, contract.UpdateNonce(class))
	got, err = contract.Nonce()
	assert.NoError(t, err)
	assert.Equal(t, class, got)

	contract.UpdateStorage([]core.StorageDiff{{addr, new(felt.Felt)}})
	sRoot, err = contract.StorageRoot()
	assert.NoError(t, err)
	assert.Equal(t, new(felt.Felt), sRoot)

	assert.NoError(t, txn.Commit())
	got, err = contract.Nonce()
	assert.Error(t, err)
}

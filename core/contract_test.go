package core

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

func TestContractAddress(t *testing.T) {
	tests := []struct {
		callerAddress       *felt.Felt
		classHash           *felt.Felt
		salt                *felt.Felt
		constructorCallData []*felt.Felt
		want                *felt.Felt
	}{
		{
			// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0
			callerAddress: utils.HexToFelt(t, "0x0000000000000000000000000000000000000000"),
			classHash: utils.HexToFelt(
				t, "0x46f844ea1a3b3668f81d38b5c1bd55e816e0373802aefe732138628f0133486"),
			salt: utils.HexToFelt(
				t, "0x74dc2fe193daf1abd8241b63329c1123214842b96ad7fd003d25512598a956b"),
			constructorCallData: []*felt.Felt{
				utils.HexToFelt(
					t,
					"0x6d706cfbac9b8262d601c38251c5fbe0497c3a96cc91a92b08d91b61d9e70c4",
				),
				utils.HexToFelt(
					t,
					"0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463",
				),
				utils.HexToFelt(t, "0x2"),
				utils.HexToFelt(
					t,
					"0x6658165b4984816ab189568637bedec5aa0a18305909c7f5726e4a16e3afef6",
				),
				utils.HexToFelt(
					t,
					"0x6b648b36b074a91eee55730f5f5e075ec19c0a8f9ffb0903cefeee93b6ff328",
				),
			},
			want: utils.HexToFelt(
				t,
				"0x3ec215c6c9028ff671b46a2a9814970ea23ed3c4bcc3838c6d1dcbf395263c3",
			),
		},
	}

	for _, tt := range tests {
		t.Run("Address", func(t *testing.T) {
			address := ContractAddress(
				tt.callerAddress,
				tt.classHash,
				tt.salt,
				tt.constructorCallData,
			)
			if !address.Equal(tt.want) {
				t.Errorf("wrong address: got %s, want %s", address.String(), tt.want.String())
			}
		})
	}
}

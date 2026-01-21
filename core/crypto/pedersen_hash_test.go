package crypto_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func TestPedersen(t *testing.T) {
	tests := []struct {
		a, b string
		want string
	}{
		{
			"0x03d937c035c878245caf64531a5756109c53068da139362728feb561405371cb",
			"0x0208a0a10250e382e1e4bbe2880906c2791bf6275695e02fbbc6aeff9cd8b31a",
			"0x030e480bed5fe53fa909cc0f8c4d99b8f9f2c016be4c41e13a4848797979c662",
		},
		{
			"0x58f580910a6ca59b28927c08fe6c43e2e303ca384badc365795fc645d479d45",
			"0x78734f65a067be9bdb39de18434d71e79f7b6466a4b66bbd979ab9e7515fe0b",
			"0x68cc0b76cddd1dd4ed2301ada9b7c872b23875d5ff837b3a87993e0d9996b87",
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("TestHash %d", i), func(t *testing.T) {
			a, err := new(felt.Felt).SetString(tt.a)
			if err != nil {
				t.Errorf("expected no error but got %s", err)
			}
			b, err := new(felt.Felt).SetString(tt.b)
			if err != nil {
				t.Errorf("expected no error but got %s", err)
			}

			want, err := new(felt.Felt).SetString(tt.want)
			if err != nil {
				t.Errorf("expected no error but got %s", err)
			}

			ans := crypto.Pedersen(a, b)
			if !ans.Equal(want) {
				t.Errorf("TestHash got %s, want %s", ans.String(), want.String())
			}
		})
	}
}

func TestPedersenArray(t *testing.T) {
	tests := [...]struct {
		input []string
		want  string
	}{
		// Contract address calculation. See the following links for how the
		// calculation is carried out and the result referenced.
		//
		// https://docs.starknet.io/architecture-and-concepts/smart-contracts/contract-address/
		{
			input: []string{
				// Hex representation of []byte("STARKNET_CONTRACT_ADDRESS").
				"0x535441524b4e45545f434f4e54524143545f41444452455353",
				// caller_address.
				"0x0",
				// salt.
				"0x5bebda1b28ba6daa824126577b9fbc984033e8b18360f5e1ef694cb172c7aa5",
				// contract_hash. See the following for reference https://alpha4.starknet.io/feeder_gateway/get_block?blockHash=0x53e61cb9a53136ecb782e7396f7330e6bb3d069763d866612da3cf93cdf55b5.
				"0x0439218681f9108b470d2379cf589ef47e60dc5888ee49ec70071671d74ca9c6",
				// calldata_hash. (here h(0, 0) where h is the Pedersen hash
				// function).
				"0x49ee3eba8c1600700ee1b87eb599f16716b0b1022947733551fde4050ca6804",
			},
			// contract_address.
			want: "0x43c6817e70b3fd99a4f120790b2e82c6843df62b573fdadf9e2d677b60ac5eb",
		},
		// Transaction hash calculation. See the following for reference.
		//
		// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=e0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75.
		{
			input: []string{
				// Hex representation of []byte("deploy").
				"0x6465706c6f79",
				// contract_address.
				"0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
				// Hex representation of keccak.Digest250([]byte("constructor")).
				"0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194",
				// calldata_hash.
				"0x7885ba4f628b6cdcd0b5e6282d2a1b17fe7cd4dd536230c5db3eac890528b4d",
				// chain_id. Hex representation of []byte("SN_MAIN").
				"0x534e5f4d41494e",
			},
			want: "0xe0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75",
		},
		// Hash of an empty array is defined to be h(0, 0).
		{
			input: make([]string, 0),
			// The value below was found using the reference implementation. See:
			// https://github.com/starkware-libs/cairo-lang/blob/de741b92657f245a50caab99cfaef093152fd8be/src/starkware/crypto/signature/fast_pedersen_hash.py#L34
			want: "0x49ee3eba8c1600700ee1b87eb599f16716b0b1022947733551fde4050ca6804",
		},
	}
	for _, test := range tests {
		var digest, digestWhole crypto.PedersenDigest
		data := make([]*felt.Felt, len(test.input))
		for i, item := range test.input {
			elem := felt.NewUnsafeFromString[felt.Felt](item)
			digest.Update(elem)
			data[i] = elem
		}
		digestWhole.Update(data...)
		want := felt.UnsafeFromString[felt.Felt](test.want)
		got := crypto.PedersenArray(data...)
		assert.Equal(t, want, got)
		assert.Equal(t, want, digest.Finish())
		assert.Equal(t, want, digestWhole.Finish())
	}
}

// By having a package and local level variable compiler optimisations can be eliminated for more accurate results.
// See here: https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go
var benchHashR felt.Felt

// go test -bench=. -run=^# -cpu=1,2,4,8,16
func BenchmarkPedersenArray(b *testing.B) {
	numOfElems := []int{3, 5, 10, 15, 20, 25, 30, 35, 40}

	for _, i := range numOfElems {
		b.Run(fmt.Sprintf("Number of felts: %d", i), func(b *testing.B) {
			randomFeltSls := genRandomFeltSls(b, i)
			var f felt.Felt
			b.ResetTimer()
			for n := range b.N {
				f = crypto.PedersenArray(randomFeltSls[n]...)
			}
			benchHashR = f
		})
	}
}

func BenchmarkPedersen(b *testing.B) {
	randFelts := genRandomFeltPairs(b)
	var f felt.Felt
	b.ResetTimer()
	for n := range b.N {
		f = crypto.Pedersen(randFelts[n][0], randFelts[n][1])
	}
	benchHashR = f
}

func genRandomFeltSls(b *testing.B, n int) [][]*felt.Felt {
	randomFeltSls := make([][]*felt.Felt, 0, b.N)
	for b.Loop() {
		randomFeltSls = append(randomFeltSls, genRandomFelts(b, n))
	}
	return randomFeltSls
}

func genRandomFelts(b *testing.B, n int) []*felt.Felt {
	b.Helper()
	felts := make([]*felt.Felt, n)
	for i := range n {
		felts[i] = felt.NewRandom[felt.Felt]()
	}
	return felts
}

func genRandomFeltPairs(b *testing.B) [][2]*felt.Felt {
	b.Helper()
	randFelts := make([][2]*felt.Felt, b.N)
	for i := range b.N {
		randFelts[i][0] = felt.NewRandom[felt.Felt]()
		randFelts[i][1] = felt.NewRandom[felt.Felt]()
	}
	return randFelts
}

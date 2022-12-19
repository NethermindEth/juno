package core

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

func TestBlockHash(t *testing.T) {
	hexToFelt := func(hex string) *felt.Felt {
		f, _ := new(felt.Felt).SetString(hex)
		return f
	}

	uintToFelt := func(uint uint) *felt.Felt {
		f := new(felt.Felt).SetUint64(uint64(uint))
		return f
	}

	tests := []struct {
		block *Block
		chain utils.Network
		want  string
	}{
		{
			// Post 0.7.0 with sequencer address
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockHash=0x40ffdbd9abbc4fc64652c50db94a29bce65c183316f304a95df624de708e746",
			&Block{
				hexToFelt("0x2e304af9a165977b79298abe812607a2d5044d278bd784f245e3cb21d7a77e8"),
				231579,
				hexToFelt("0x1ee483d84c82fec55ec52fdf62e85abaebc47dfe0e4623187a2350a17a1b1dc"),
				hexToFelt("0x46a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b"),
				uintToFelt(1654526121),
				uintToFelt(65),
				hexToFelt("0x73a0e2053e3ab5c3e23f656bdb8c6055e572174426a9453db4a486835cfd596"),
				uintToFelt(89),
				hexToFelt("0x125a4eebce8aa3f9f0825c15d93a06f0977d55799aa2917d040bffe30ac444a"),
				uintToFelt(0),
				hexToFelt(""),
			},
			0,
			"0x40ffdbd9abbc4fc64652c50db94a29bce65c183316f304a95df624de708e746",
		},
		{
			// Post 0.7.0 without sequencer address
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=156000",
			&Block{
				hexToFelt("0x331e6b9d99341aba27113ff30bd211b84194e87f2a8fe41f3485ca91b3e047b"),
				156000,
				hexToFelt("0x24e7360800ca4cdfc0ac3e18fb32399142d75b7a20d29ecbb563fbf962aa3c5"),
				nil,
				uintToFelt(1649872212),
				uintToFelt(29),
				hexToFelt("0x24638e0ca122d0260d54e901dc0942ea68bd1fc40a96b5da765985c47c92500"),
				uintToFelt(55),
				hexToFelt("0x5d25e41d43b00681cc63ed4e13a82efe3e02f47e03173efbd737dd52ba88c7e"),
				uintToFelt(0),
				hexToFelt(""),
			},
			0,
			"0x1288267b119adefd52795c3421f8fabba78f49e911f39c1fb2f4e5eb8fb771",
		},
		{
			// block 1: pre 0.7.0
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=1",
			&Block{
				hexToFelt("0x7d328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b"),
				1,
				hexToFelt("0x3f04ffa63e188d602796505a2ee4f6e1f294ee29a914b057af8e75b17259d9f"),
				hexToFelt(""),
				uintToFelt(1636989916),
				uintToFelt(4),
				hexToFelt("0x18bb7d6c1c558aa0a025f08a7d723a44b13008ffb444c432077f319a7f4897c"),
				uintToFelt(0),
				hexToFelt(""),
				uintToFelt(0),
				hexToFelt(""),
			},
			0,
			"0x75e00250d4343326f322e370df4c9c73c7be105ad9f532eeb97891a34d9e4a5",
		},
	}

	for _, tt := range tests {
		got, err := tt.block.Hash(tt.chain)
		if err != nil {
			t.Fatal(err)
		}
		if "0x"+(got.Text(16)) != tt.want {
			t.Errorf("got %s, want %s", "0x"+got.Text(16), tt.want)
		}
	}
}

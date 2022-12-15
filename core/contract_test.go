package core

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
)

var (
	//go:embed testdata/bytecode_genesis.json
	bytecodeGenesisBytes []byte
	//go:embed testdata/bytecode_0_8.json
	bytecodeCairo08Bytes []byte
	//go:embed testdata/bytecode_0_10.json
	bytecodeCairo10Bytes []byte
)

func TestClassHash(t *testing.T) {
	var bytecodeGenesis []*felt.Felt
	if err := json.Unmarshal(bytecodeGenesisBytes, &bytecodeGenesis); err != nil {
		t.Fatalf("unexpected error while unmarshalling bytecodeGenesisBytes: %s", err)
	}
	var bytecodeCairo08 []*felt.Felt
	if err := json.Unmarshal(bytecodeCairo08Bytes, &bytecodeCairo08); err != nil {
		t.Fatalf("unexpected error while unmarshalling bytecodeCairo08Bytes: %s", err)
	}
	var bytecodeCairo10 []*felt.Felt
	if err := json.Unmarshal(bytecodeCairo10Bytes, &bytecodeCairo10); err != nil {
		t.Fatalf("unexpected error while unmarshalling bytecodeBytes: %s", err)
	}

	// We know our test hex values are valid, so we'll ignore the potential error
	hexToFelt := func(hex string) *felt.Felt {
		f, _ := new(felt.Felt).SetString(hex)
		return f
	}

	tests := []struct {
		class *Class
		want  *felt.Felt
	}{
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8
			class: &Class{
				APIVersion: new(felt.Felt),
				Externals: []EntryPoint{
					{
						Offset:   hexToFelt("0x40a"),
						Selector: hexToFelt("0x5fbd85570830519219bb4ad6951316f96fce363f86909d1f8adb1fdc836471"),
					},
					{
						Offset:   hexToFelt("0x153"),
						Selector: hexToFelt("0x7772be8b80a8a33dc6c1f9a6ab820c02e537c73e859de67f288c70f92571bb"),
					},
					{
						Offset:   hexToFelt("0x3a3"),
						Selector: hexToFelt("0x8a2a3272a92492ded6c04f7c85df9c53134cef398564465f12af3c9c986d41"),
					},
					{
						Offset:   hexToFelt("0x313"),
						Selector: hexToFelt("0xbd7daa40535813d892224da817610f4c7e6fe8983abe588a4227586262d9d3"),
					},
					{
						Offset:   hexToFelt("0x2e5"),
						Selector: hexToFelt("0xe8f69bd941db5b0bff2e416c63d46f067fcdfad558c528f9fd102ba368cb5f"),
					},
					{
						Offset:   hexToFelt("0x25d"),
						Selector: hexToFelt("0x12ead94ae9d3f9d2bdb6b847cf255f1f398193a1f88884a0ae8e18f24a037b6"),
					},
					{
						Offset:   hexToFelt("0x347"),
						Selector: hexToFelt("0x19a35a6e95cb7a3318dbb244f20975a1cd8587cc6b5259f15f61d7beb7ee43b"),
					},
					{
						Offset:   hexToFelt("0x1b3"),
						Selector: hexToFelt("0x1ae1a515cf2d214b29bdf63a79ee2d490efd4dd1acc99d383a8e549c3cecb5d"),
					},
					{
						Offset:   hexToFelt("0x3cc"),
						Selector: hexToFelt("0x1b1343fe0f4a16bed5e5133b5ca9f03ab15976bb2df2b6d263ac3170b8b6a13"),
					},
					{
						Offset:   hexToFelt("0x212"),
						Selector: hexToFelt("0x1cad42b55a5b2c7366b371db59448730766dfef74c0156c9c6f332c8c5e34d9"),
					},
					{
						Offset:   hexToFelt("0x23e"),
						Selector: hexToFelt("0x1eaab699414d786ce9dbfd4e86815f66680647efd13f9334ac97148e4e30e82"),
					},
					{
						Offset:   hexToFelt("0x37a"),
						Selector: hexToFelt("0x218f305395474a84a39307fa5297be118fe17bf65e27ac5e2de6617baa44c64"),
					},
					{
						Offset:   hexToFelt("0x1ec"),
						Selector: hexToFelt("0x26813d396fdb198e9ead934e4f7a592a8b88a059e45ab0eb6ee53494e8d45b0"),
					},
					{
						Offset:   hexToFelt("0x277"),
						Selector: hexToFelt("0x27c3334165536f239cfd400ed956eabff55fc60de4fb56728b6a4f6b87db01c"),
					},
					{
						Offset:   hexToFelt("0x29c"),
						Selector: hexToFelt("0x29cef374bfc7ad2628f04d9a18ac3c3a259c1eb3ce3d3c77bbab281c42649fc"),
					},
					{
						Offset:   hexToFelt("0x187"),
						Selector: hexToFelt("0x30f842021fbf02caf80d09a113997c1e00a32870eee0c6136bed27acb348bea"),
					},
					{
						Offset:   hexToFelt("0x100"),
						Selector: hexToFelt("0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f"),
					},
					{
						Offset:   hexToFelt("0x2fc"),
						Selector: hexToFelt("0x33ce93a3eececa5c9fc70da05f4aff3b00e1820b79587924d514bc76788991a"),
					},
					{
						Offset:   hexToFelt("0x3ea"),
						Selector: hexToFelt("0x34c4c150632e67baf44fc50e9a685184d72a822510a26a66f72058b5e7b2892"),
					},
					{
						Offset:   hexToFelt("0x1cc"),
						Selector: hexToFelt("0x3d7905601c217734671143d457f0db37f7f8883112abd34b92c4abfeafde0c3"),
					},
				},
				L1Handlers: []EntryPoint{
					{
						Offset:   hexToFelt("0x2cb"),
						Selector: hexToFelt("0xc73f681176fc7b3f9693986fd7b14581e8d540519e27400e88b8713932be01"),
					},
				},
				Constructors: []EntryPoint{
					{
						Offset:   hexToFelt("0x125"),
						Selector: hexToFelt("0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194"),
					},
				},
				Builtins: []*felt.Felt{
					new(felt.Felt).SetBytes([]byte("pedersen")),
					new(felt.Felt).SetBytes([]byte("range_check")),
					new(felt.Felt).SetBytes([]byte("bitwise")),
				},
				ProgramHash: hexToFelt("0x1e87d79be8c8146494b5c54318f7d194481c3959752659a1e1bce158649a670"),
				Bytecode:    bytecodeGenesis,
			},
			want: hexToFelt("0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"),
		},
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3
			class: &Class{
				APIVersion: new(felt.Felt),
				Externals: []EntryPoint{
					{
						Offset:   hexToFelt("0xed"),
						Selector: hexToFelt("0x1a35984e05126dbecb7c3bb9929e7dd9106d460c59b1633739a5c733a5fb13b"),
					},
					{
						Offset:   hexToFelt("0x10c"),
						Selector: hexToFelt("0x1ac47721ee58ba2813c2a816bca188512839a00d3970f67c05eab986b14006d"),
					},
					{
						Offset:   hexToFelt("0x1bd"),
						Selector: hexToFelt("0x240060cdb34fcc260f41eac7474ee1d7c80b7e3607daff9ac67c7ea2ebb1c44"),
					},
					{
						Offset:   hexToFelt("0x163"),
						Selector: hexToFelt("0x28420862938116cb3bbdbedee07451ccc54d4e9412dbef71142ad1980a30941"),
					},
					{
						Offset:   hexToFelt("0xce"),
						Selector: hexToFelt("0x2de154d8a89be65c1724e962dc4c65637c05532a6c2825d0a7b7d774169dbba"),
					},
					{
						Offset:   hexToFelt("0x125"),
						Selector: hexToFelt("0x2e3e21ff5952b2531241e37999d9c4c8b3034cccc89a202a6bf019bdf5294f9"),
					},
				},
				L1Handlers: make([]EntryPoint, 0),
				Constructors: []EntryPoint{
					{
						Offset:   hexToFelt("0x13f"),
						Selector: hexToFelt("0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194"),
					},
				},
				Builtins: []*felt.Felt{
					new(felt.Felt).SetBytes([]byte("pedersen")),
					new(felt.Felt).SetBytes([]byte("range_check")),
					new(felt.Felt).SetBytes([]byte("ecdsa")),
				},
				ProgramHash: hexToFelt("0x359145fc6207854bfbbeadae4c6e289024400a5af87090ed18073200fef6213"),
				Bytecode:    bytecodeCairo08,
			},
			want: hexToFelt("0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3"),
		},
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118
			class: &Class{
				APIVersion: new(felt.Felt),
				Externals: []EntryPoint{
					{
						Offset:   hexToFelt("0x3a"),
						Selector: hexToFelt("0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320"),
					},
					{
						Offset:   hexToFelt("0x5b"),
						Selector: hexToFelt("0x39e11d48192e4333233c7eb19d10ad67c362bb28580c604d67884c85da39695"),
					},
				},
				L1Handlers: make([]EntryPoint, 0),
				Constructors: []EntryPoint{
					{
						Offset:   hexToFelt("0x71"),
						Selector: hexToFelt("0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194"),
					},
				},
				Builtins: []*felt.Felt{
					new(felt.Felt).SetBytes([]byte("pedersen")),
					new(felt.Felt).SetBytes([]byte("range_check")),
				},
				ProgramHash: hexToFelt("0x88562ac88adfc7760ff452d048d39d72978bcc0f8d7b0fcfb34f33970b3df3"),
				Bytecode:    bytecodeCairo10,
			},
			want: hexToFelt("0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118"),
		},
	}

	for _, tt := range tests {
		t.Run("ClassHash", func(t *testing.T) {
			classHash := tt.class.Hash()
			if !classHash.Equal(tt.want) {
				t.Errorf("wrong hash: got %s, want %s", classHash.Text(16), tt.want.Text(16))
			}
		})
	}
}

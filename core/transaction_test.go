package core

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/test-go/testify/assert"
)

var (
	//go:embed testdata/bytecode_v0_declare.json
	bytecodeV0DeclareTransBytes []byte
	//go:embed testdata/bytecode_v1_declare.json
	bytecodeV1DeclareTransBytes []byte
)

// We know our test hex values are valid, so we'll ignore the potential error
func hexToFelt(hex string) *felt.Felt {
	f, _ := new(felt.Felt).SetString(hex)
	return f
}

func TestDeployTransactions(t *testing.T) {
	tests := map[string]struct {
		input DeployTransaction
		want  *felt.Felt
	}{
		"Deploy transaction": {input: DeployTransaction{
			ContractAddress:     hexToFelt("0x3ec215c6c9028ff671b46a2a9814970ea23ed3c4bcc3838c6d1dcbf395263c3"),
			ContractAddressSalt: hexToFelt("0x74dc2fe193daf1abd8241b63329c1123214842b96ad7fd003d25512598a956b"),
			Class:               Class{},
			ConstructorCalldata: [](*felt.Felt){
				hexToFelt("0x6d706cfbac9b8262d601c38251c5fbe0497c3a96cc91a92b08d91b61d9e70c4"),
				hexToFelt("0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463"),
				hexToFelt("0x2"),
				hexToFelt("0x6658165b4984816ab189568637bedec5aa0a18305909c7f5726e4a16e3afef6"),
				hexToFelt("0x6b648b36b074a91eee55730f5f5e075ec19c0a8f9ffb0903cefeee93b6ff328"),
			},
			CallerAddress: new(felt.Felt).SetUint64(0),
			Version:       new(felt.Felt).SetUint64(0),
		}, want: hexToFelt("0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0")}}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			transactionHash, err := test.input.Hash([]byte("SN_MAIN"))
			assert.Nil(t, err, "expected no error but got %s", err)
			assert.Equal(t, test.want, transactionHash, "Transaction Hash got %s, want %s", transactionHash.Text(16), test.want.Text(16))
		})
	}
}

func TestInvokeTransactions(t *testing.T) {
	tests := map[string]struct {
		input InvokeTransaction
		want  *felt.Felt
	}{
		"Invoke transaction version 0": {
			input: InvokeTransaction{
				ContractAddress:    hexToFelt("0x43324c97e376d7d164abded1af1e73e9ce8214249f711edb7059c1ca34560e8"),
				EntryPointSelector: hexToFelt("0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f"),
				CallData: [](*felt.Felt){
					hexToFelt("0x1b654cb59f978da2eee76635158e5ff1399bf607cb2d05e3e3b4e41d7660ca2"),
					hexToFelt("0x2"),
					hexToFelt("0x5f743efdb29609bfc2002041bdd5c72257c0c6b5c268fc929a3e516c171c731"),
					hexToFelt("0x635afb0ea6c4cdddf93f42287b45b67acee4f08c6f6c53589e004e118491546"),
				},
				MaxFee:  hexToFelt("0x0"),
				Version: new(felt.Felt).SetUint64(0),
			},
			want: hexToFelt("0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e"),
		},
		"Invoke transaction version 1": {
			input: InvokeTransaction{
				ContractAddress: hexToFelt("0x3ec215c6c9028ff671b46a2a9814970ea23ed3c4bcc3838c6d1dcbf395263c3"),
				CallData: [](*felt.Felt){
					hexToFelt("0x1"),
					hexToFelt("0x727a63f78ee3f1bd18f78009067411ab369c31dece1ae22e16f567906409905"),
					hexToFelt("0x22de356837ac200bca613c78bd1fcc962a97770c06625f0c8b3edeb6ae4aa59"),
					hexToFelt("0x0"),
					hexToFelt("0xb"),
					hexToFelt("0xb"),
					hexToFelt("0xa"),
					hexToFelt("0x6db793d93ce48bc75a5ab02e6a82aad67f01ce52b7b903090725dbc4000eaa2"),
					hexToFelt("0x6141eac4031dfb422080ed567fe008fb337b9be2561f479a377aa1de1d1b676"),
					hexToFelt("0x27eb1a21fa7593dd12e988c9dd32917a0dea7d77db7e89a809464c09cf951c0"),
					hexToFelt("0x400a29400a34d8f69425e1f4335e6a6c24ce1111db3954e4befe4f90ca18eb7"),
					hexToFelt("0x599e56821170a12cdcf88fb8714057ce364a8728f738853da61d5b3af08a390"),
					hexToFelt("0x46ad66f467df625f3b2dd9d3272e61713e8f74b68adac6718f7497d742cfb17"),
					hexToFelt("0x4f348b585e6c1919d524a4bfe6f97230ecb61736fe57534ec42b628f7020849"),
					hexToFelt("0x19ae40a095ffe79b0c9fc03df2de0d2ab20f59a2692ed98a8c1062dbf691572"),
					hexToFelt("0xe120336994adef6c6e47694f87278686511d4622997d4a6f216bd6e9fa9acc"),
					hexToFelt("0x56e6637a4958d062db8c8198e315772819f64d915e5c7a8d58a99fa90ff0742"),
				},
				Signature: [](*felt.Felt){
					hexToFelt("0x383ba105b6d0f59fab96a412ad267213ddcd899e046278bdba64cd583d680b"),
					hexToFelt("0x1896619a17fde468978b8d885ffd6f5c8f4ac1b188233b81b91bcf7dbc56fbd"),
				},
				Nonce:         hexToFelt("0x42"),
				SenderAddress: hexToFelt("0x1fc039de7d864580b57a575e8e6b7114f4d2a954d7d29f876b2eb3dd09394a0"),
				MaxFee:        hexToFelt("0x17f0de82f4be6"),
				Version:       new(felt.Felt).SetUint64(1),
			},
			want: hexToFelt("0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			transactionHash, err := test.input.Hash([]byte("SN_MAIN"))
			assert.Nil(t, err, "expected no error but got %s", err)
			assert.Equal(t, test.want, transactionHash, "Transaction Hash got %s, want %s", transactionHash.Text(16), test.want.Text(16))
		})
	}
}

func TestDeclareTransaction(t *testing.T) {
	var bytecodeV0Declare []*felt.Felt
	if err := json.Unmarshal(bytecodeV0DeclareTransBytes, &bytecodeV0Declare); err != nil {
		t.Fatalf("unexpected error while unmarshalling bytecodeBytes: %s", err)
	}

	var bytecodeV1Declare []*felt.Felt
	if err := json.Unmarshal(bytecodeV1DeclareTransBytes, &bytecodeV1Declare); err != nil {
		t.Fatalf("unexpected error while unmarshalling bytecodeBytes: %s", err)
	}

	tests := map[string]struct {
		input DeclareTransaction
		want  *felt.Felt
	}{
		"Declare transaction version 0": {
			input: DeclareTransaction{
				Class: Class{
					APIVersion: new(felt.Felt),
					Externals: []EntryPoint{
						{Offset: hexToFelt("0x67b"), Selector: hexToFelt("0x151e58b29179122a728eab07c8847e5baf5802379c5db3a7d57a8263a7bd1d")},
						{Offset: hexToFelt("0x590"), Selector: hexToFelt("0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20")},
						{Offset: hexToFelt("0x2df"), Selector: hexToFelt("0x4c4fb1ab068f6039d5780c68dd0fa2f8742cceb3426d19667778ca7f3518a9")},
						{Offset: hexToFelt("0x50b"), Selector: hexToFelt("0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463")},
						{Offset: hexToFelt("0x2c1"), Selector: hexToFelt("0x80aa9fdbfaf9615e4afc7f5f722e265daca5ccc655360fa5ccacf9c267936d")},
						{Offset: hexToFelt("0x53e"), Selector: hexToFelt("0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e")},
						{Offset: hexToFelt("0x6a0"), Selector: hexToFelt("0xd63a78e4cd7fb4c41bc18d089154af78d400a5e837f270baea6cf8db18c8dd")},
						{Offset: hexToFelt("0x605"), Selector: hexToFelt("0x16cc063b8338363cf388ce7fe1df408bf10f16cd51635d392e21d852fafb683")},
						{Offset: hexToFelt("0x656"), Selector: hexToFelt("0x1aaf3e6107dd1349c81543ff4221a326814f77dadcc5810807b74f1a49ded4e")},
						{Offset: hexToFelt("0x323"), Selector: hexToFelt("0x1e888a1026b19c8c0b57c72d63ed1737106aa10034105b980ba117bd0c29fe1")},
						{Offset: hexToFelt("0x2a2"), Selector: hexToFelt("0x216b05c387bab9ac31918a3e61672f4618601f3c598a2f3f2710f37053e1ea4")},
						{Offset: hexToFelt("0x5bd"), Selector: hexToFelt("0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c")},
						{Offset: hexToFelt("0x4d7"), Selector: hexToFelt("0x2a4bb4205277617b698a9a2950b938d0a236dd4619f82f05bec02bdbd245fab")},
						{Offset: hexToFelt("0x4ef"), Selector: hexToFelt("0x2c4943a27e820803a6ef49bb04b629950e2de615ab9ac0fb8baef037b168782")},
						{Offset: hexToFelt("0x2ff"), Selector: hexToFelt("0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e")},
						{Offset: hexToFelt("0x45c"), Selector: hexToFelt("0x358a2fe57368393087d3e6d24f1e04741c5bdc85e3e23790253e377f55c391e")},
						{Offset: hexToFelt("0x284"), Selector: hexToFelt("0x361458367e696363fbcc70777d07ebbd2394e89fd0adcaf147faccd1d294d60")},
						{Offset: hexToFelt("0x4a7"), Selector: hexToFelt("0x3c0ba99f1a18bcdc81fcbcb6b4f15a9a6725f937075aed6fac107ffcb147068")},
					},
					L1Handlers:   make([]EntryPoint, 0),
					Constructors: make([]EntryPoint, 0),
					Builtins: []*felt.Felt{
						new(felt.Felt).SetBytes([]byte("pedersen")),
						new(felt.Felt).SetBytes([]byte("range_check")),
					},
					ProgramHash: hexToFelt("0x3da27c3b03fd2a8c47c200b78779eac4c4a8848151b3c9f3343dc27f7df6c68"),
					Bytecode:    bytecodeV0Declare,
				},
				Nonce:         hexToFelt("0x0"),
				SenderAddress: hexToFelt("0x1"),
				MaxFee:        hexToFelt("0x0"),
				Version:       new(felt.Felt).SetUint64(0),
			},
			want: hexToFelt("0x222f8902d1eeea76fa2642a90e2411bfd71cffb299b3a299029e1937fab3fe4"),
		},
		"Declare transaction version 1": {
			input: DeclareTransaction{
				Class: Class{
					APIVersion: new(felt.Felt),
					Externals: []EntryPoint{
						{Selector: hexToFelt("0x4bc30b413bff05d0c5a841b495efaa7136b1150650a6d50b5125348ef247b7"), Offset: hexToFelt("0x8cc")},
						{Selector: hexToFelt("0xd399e873cde9f1130182a2b70db45e021df5a2f404fa14e8b2f7481c10f1d3"), Offset: hexToFelt("0x636")},
						{Selector: hexToFelt("0xe323be32563ff4ec7eb6dbb44de6f462fe50d4347d2a5ed97b8000b01fae0f"), Offset: hexToFelt("0x96e")},
						{Selector: hexToFelt("0x1746ba88936c89803398e3d705b7c60b975ba95ed8309170af08a50cfd76034"), Offset: hexToFelt("0x887")},
						{Selector: hexToFelt("0x1a5b9e3dbe4d6bd046be696b63482af53d74488393698b847f9fdba7ab3eb84"), Offset: hexToFelt("0x564")},
						{Selector: hexToFelt("0x1d481eb4b63b94bb55e6b98aabb06c3b8484f82a4d656d6bca0b0cf9b446be0"), Offset: hexToFelt("0x5be")},
						{Selector: hexToFelt("0x1fecea0123deaf1ec6b125dfb3ebb67d43ae1820f1cfddfd04748c2bb8723f2"), Offset: hexToFelt("0x93c")},
						{Selector: hexToFelt("0x1ff283d9ff7e020d53bd8df3046dbc2f9844949ba7aca7f74053ac71f270ffb"), Offset: hexToFelt("0x672")},
						{Selector: hexToFelt("0x2016836a56b71f0d02689e69e326f4f4c1b9057164ef592671cf0d37c8040c0"), Offset: hexToFelt("0x690")},
						{Selector: hexToFelt("0x2635814066a61a91118251ce2c36965c1c8bd482f624471790044f16545fd67"), Offset: hexToFelt("0x800")},
						{Selector: hexToFelt("0x2be9875f83b420d4b53991a231f8c1b1e97f63799a9518d9fa4714e1a194c62"), Offset: hexToFelt("0x902")},
						{Selector: hexToFelt("0x2dd76e7ad84dbed81c314ffe5e7a7cacfb8f4836f01af4e913f275f89a3de1a"), Offset: hexToFelt("0x539")},
						{Selector: hexToFelt("0x33b9f6abf0b529613680afe2a00fa663cc95cbdc47d726d85a044462eabbf02"), Offset: hexToFelt("0x654")},
						{Selector: hexToFelt("0x3430195ba4c8865b3e4ece998a22147a09e701161f2e28c0e3b6fd48f22238d"), Offset: hexToFelt("0x99d")},
						{Selector: hexToFelt("0x357029217c9f3f982fdd88896cec398a11c800ec3008aae973b40c056b6162f"), Offset: hexToFelt("0x74b")},
						{Selector: hexToFelt("0x35d7d17c374504dc9e7e19f5d8202b1c11bc59642d4abc65b2373426bea2105"), Offset: hexToFelt("0x9df")},
						{Selector: hexToFelt("0x366a98476020cb9ff8cc566d0cdeac414e546d2e7ede445f4e7032a4272c771"), Offset: hexToFelt("0x582")},
						{Selector: hexToFelt("0x36a0899cf87fd4dc1049f3ec4d9d2ab52bbb0c9a3fcdd483f6c38a906da8f51"), Offset: hexToFelt("0x788")},
						{Selector: hexToFelt("0x39b0454cadcb5884dd3faa6ba975da4d2459aa3f11d31291a25a8358f84946d"), Offset: hexToFelt("0x5fa")},
						{Selector: hexToFelt("0x39db8947b7337181fb15cc84373df7e708a276840ebe0927e9a12b86ff38aa3"), Offset: hexToFelt("0x6ec")},
						{Selector: hexToFelt("0x3b904aa5afc486c58c0b51ae01374c1c29068417feb59eaeecf86ac46c1fef9"), Offset: hexToFelt("0x5a0")},
						{Selector: hexToFelt("0x3da9c62205655e202173ec115b91229a1afafeb0329c0797d4a94c5d5de80fa"), Offset: hexToFelt("0x618")},
						{Selector: hexToFelt("0x3e75033db4684c97865a0e4372cf714e5bad6437ec2e2d7b693019d0661f9ee"), Offset: hexToFelt("0x5dc")},
					},
					L1Handlers:   make([]EntryPoint, 0),
					Constructors: make([]EntryPoint, 0),
					Builtins: []*felt.Felt{
						new(felt.Felt).SetBytes([]byte("pedersen")),
						new(felt.Felt).SetBytes([]byte("range_check")),
					},
					ProgramHash: hexToFelt("0x1f2c4b0f3fb0e1e30308b0d1dc58131d4f82b2a0df1bf637179f5754abee13a"),
					Bytecode:    bytecodeV1Declare,
				},
				Signature: [](*felt.Felt){
					hexToFelt("0x221b9576c4f7b46d900a331d89146dbb95a7b03d2eb86b4cdcf11331e4df7f2"),
					hexToFelt("0x667d8062f3574ba9b4965871eec1444f80dacfa7114e1d9c74662f5672c0620"),
				},
				Nonce:         hexToFelt("0x5"),
				SenderAddress: hexToFelt("0x39291faa79897de1fd6fb1a531d144daa1590d058358171b83eadb3ceafed8"),
				MaxFee:        hexToFelt("0xf6dbd653833"),
				Version:       new(felt.Felt).SetUint64(1),
			},
			want: hexToFelt("0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae")},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			transactionHash, err := test.input.Hash([]byte("SN_MAIN"))
			assert.Nil(t, err, "expected no error but got %s", err)
			assert.Equal(t, test.want, transactionHash, "Transaction Hash got %s, want %s", transactionHash.Text(16), test.want.Text(16))
		})
	}
}

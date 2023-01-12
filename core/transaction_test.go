package core

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
)

var (
	//go:embed testdata/bytecode_declare.json
	bytecodeDeclareTransBytes []byte
)

func TestDeployTransactions(t *testing.T) {
	class := Class{}

	contractAddressSalt, _ := new(felt.Felt).SetString("0x74dc2fe193daf1abd8241b63329c1123214842b96ad7fd003d25512598a956b")
	contractAddress, _ := new(felt.Felt).SetString("0x3ec215c6c9028ff671b46a2a9814970ea23ed3c4bcc3838c6d1dcbf395263c3")
	callData1, _ := new(felt.Felt).SetString("0x6d706cfbac9b8262d601c38251c5fbe0497c3a96cc91a92b08d91b61d9e70c4")
	callData2, _ := new(felt.Felt).SetString("0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463")
	callData3, _ := new(felt.Felt).SetString("0x2")
	callData4, _ := new(felt.Felt).SetString("0x6658165b4984816ab189568637bedec5aa0a18305909c7f5726e4a16e3afef6")
	callData5, _ := new(felt.Felt).SetString("0x6b648b36b074a91eee55730f5f5e075ec19c0a8f9ffb0903cefeee93b6ff328")

	DeployTransactionObj := DeployTransaction{
		ContractAddressSalt: contractAddressSalt,
		ContractAddress:     contractAddress,
		Class:               class,
		ConstructorCalldata: [](*felt.Felt){
			callData1,
			callData2,
			callData3,
			callData4,
			callData5,
		},
		CallerAddress: new(felt.Felt).SetUint64(0),
		Version:       new(felt.Felt).SetUint64(0),
	}

	t.Run("DeployTransactionHash", func(t *testing.T) {
		transactionHash, err := DeployTransactionObj.Hash([]byte("SN_MAIN"))
		if err != nil {
			t.Fatalf("expected no error but got %s", err)
		}
		fmt.Println("Transaction Hash: ", transactionHash.Text(16))
		expectedHash, _ := new(felt.Felt).SetString("0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0")
		if !transactionHash.Equal(expectedHash) {
			t.Errorf("Transaction Hash got %s, want %s", transactionHash.Text(16), expectedHash.Text(16))
		}
	})
}

func TestInvokeTransactions(t *testing.T) {
	contractAddress, _ := new(felt.Felt).SetString("0x1fc039de7d864580b57a575e8e6b7114f4d2a954d7d29f876b2eb3dd09394a0")
	callData1, _ := new(felt.Felt).SetString("0x1")
	callData2, _ := new(felt.Felt).SetString("0x727a63f78ee3f1bd18f78009067411ab369c31dece1ae22e16f567906409905")
	callData3, _ := new(felt.Felt).SetString("0x22de356837ac200bca613c78bd1fcc962a97770c06625f0c8b3edeb6ae4aa59")
	callData4, _ := new(felt.Felt).SetString("0x0")
	callData5, _ := new(felt.Felt).SetString("0xb")
	callData6, _ := new(felt.Felt).SetString("0xb")
	callData7, _ := new(felt.Felt).SetString("0xa")
	callData8, _ := new(felt.Felt).SetString("0x6db793d93ce48bc75a5ab02e6a82aad67f01ce52b7b903090725dbc4000eaa2")
	callData9, _ := new(felt.Felt).SetString("0x6141eac4031dfb422080ed567fe008fb337b9be2561f479a377aa1de1d1b676")
	callData10, _ := new(felt.Felt).SetString("0x27eb1a21fa7593dd12e988c9dd32917a0dea7d77db7e89a809464c09cf951c0")
	callData11, _ := new(felt.Felt).SetString("0x400a29400a34d8f69425e1f4335e6a6c24ce1111db3954e4befe4f90ca18eb7")
	callData12, _ := new(felt.Felt).SetString("0x599e56821170a12cdcf88fb8714057ce364a8728f738853da61d5b3af08a390")
	callData13, _ := new(felt.Felt).SetString("0x46ad66f467df625f3b2dd9d3272e61713e8f74b68adac6718f7497d742cfb17")
	callData14, _ := new(felt.Felt).SetString("0x4f348b585e6c1919d524a4bfe6f97230ecb61736fe57534ec42b628f7020849")
	callData15, _ := new(felt.Felt).SetString("0x19ae40a095ffe79b0c9fc03df2de0d2ab20f59a2692ed98a8c1062dbf691572")
	callData16, _ := new(felt.Felt).SetString("0xe120336994adef6c6e47694f87278686511d4622997d4a6f216bd6e9fa9acc")
	callData17, _ := new(felt.Felt).SetString("0x56e6637a4958d062db8c8198e315772819f64d915e5c7a8d58a99fa90ff0742")

	senderAddress, _ := new(felt.Felt).SetString("0x1fc039de7d864580b57a575e8e6b7114f4d2a954d7d29f876b2eb3dd09394a0")

	maxFee, _ := new(felt.Felt).SetString("0x17f0de82f4be6")
	nonce, _ := new(felt.Felt).SetString("0x42")

	signature1, _ := new(felt.Felt).SetString("0x383ba105b6d0f59fab96a412ad267213ddcd899e046278bdba64cd583d680b")
	signature2, _ := new(felt.Felt).SetString("0x1896619a17fde468978b8d885ffd6f5c8f4ac1b188233b81b91bcf7dbc56fbd")

	InvokeTransactionObj := InvokeTransaction{
		ContractAddress: contractAddress,
		Nonce:           nonce,
		SenderAddress:   senderAddress,
		CallData: [](*felt.Felt){
			callData1,
			callData2,
			callData3,
			callData4,
			callData5,
			callData6,
			callData7,
			callData8,
			callData9,
			callData10,
			callData11,
			callData12,
			callData13,
			callData14,
			callData15,
			callData16,
			callData17,
		},
		Signature: [](*felt.Felt){
			signature1,
			signature2,
		},
		MaxFee:  maxFee,
		Version: new(felt.Felt).SetUint64(1),
	}

	t.Run("InvokeTransactionHash", func(t *testing.T) {
		transactionHash, err := InvokeTransactionObj.Hash([]byte("SN_MAIN"))
		if err != nil {
			t.Fatalf("expected no error but got %s", err)
		}
		fmt.Println("Transaction Hash: ", transactionHash.Text(16))
		expectedHash, _ := new(felt.Felt).SetString("0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497")
		if !transactionHash.Equal(expectedHash) {
			t.Errorf("Transaction Hash got %s, want %s", transactionHash.Text(16), expectedHash.Text(16))
		}
	})
}

func TestDeclareTransaction(t *testing.T) {
	var bytecodeDeclare []*felt.Felt
	if err := json.Unmarshal(bytecodeDeclareTransBytes, &bytecodeDeclare); err != nil {
		t.Fatalf("unexpected error while unmarshalling bytecodeBytes: %s", err)
	}

	// We know our test hex values are valid, so we'll ignore the potential error
	hexToFelt := func(hex string) *felt.Felt {
		f, _ := new(felt.Felt).SetString(hex)
		return f
	}

	class := Class{
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
		Bytecode:    bytecodeDeclare,
	}

	senderAddress, _ := new(felt.Felt).SetString("0x39291faa79897de1fd6fb1a531d144daa1590d058358171b83eadb3ceafed8")
	maxFee, _ := new(felt.Felt).SetString("0xf6dbd653833")
	nonce, _ := new(felt.Felt).SetString("0x5")
	signature1, _ := new(felt.Felt).SetString("0x221b9576c4f7b46d900a331d89146dbb95a7b03d2eb86b4cdcf11331e4df7f2")
	signature2, _ := new(felt.Felt).SetString("0x667d8062f3574ba9b4965871eec1444f80dacfa7114e1d9c74662f5672c0620")
	classHash, _ := new(felt.Felt).SetString("0x7aed6898458c4ed1d720d43e342381b25668ec7c3e8837f761051bf4d655e54")
	fmt.Println("Expected class hash:", classHash)
	version, _ := new(felt.Felt).SetString("0x1")
	DeclareTransactionObj := DeclareTransaction{
		Class:         class,
		SenderAddress: senderAddress,
		MaxFee:        maxFee,
		Signature: [](*felt.Felt){
			signature1,
			signature2,
		},
		Nonce:   nonce,
		Version: version,
	}

	t.Run("DeclareTransactionHash", func(t *testing.T) {
		transactionHash, err := DeclareTransactionObj.Hash([]byte("SN_MAIN"))
		if err != nil {
			t.Fatalf("expected no error but got %s", err)
		}
		expectedHash, _ := new(felt.Felt).SetString("0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae")
		if !transactionHash.Equal(expectedHash) {
			t.Errorf("Transaction Hash got %s, want %s", transactionHash.Text(16), expectedHash.Text(16))
		}
	})
}

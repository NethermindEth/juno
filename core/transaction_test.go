package core

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/NethermindEth/juno/core/contract"
	"github.com/NethermindEth/juno/core/felt"
)

func TestDeployTransactions(t *testing.T) {
	// Read json file, parse it and generate a class
	contractDefinition, err := ioutil.ReadFile("TestData/class_definition_deploy.json")
	if err != nil {
		t.Fatalf("expected no error but got %s", err)
	}
	c, err := contract.GenerateClass(contractDefinition)
	if err != nil {
		t.Fatalf("expected no error but got %s", err)
	}

	contractAddressSalt, _ := new(felt.Felt).SetString("0x74dc2fe193daf1abd8241b63329c1123214842b96ad7fd003d25512598a956b")
	callData1, _ := new(felt.Felt).SetString("0x6d706cfbac9b8262d601c38251c5fbe0497c3a96cc91a92b08d91b61d9e70c4")
	callData2, _ := new(felt.Felt).SetString("0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463")
	callData3, _ := new(felt.Felt).SetString("0x2")
	callData4, _ := new(felt.Felt).SetString("0x6658165b4984816ab189568637bedec5aa0a18305909c7f5726e4a16e3afef6")
	callData5, _ := new(felt.Felt).SetString("0x6b648b36b074a91eee55730f5f5e075ec19c0a8f9ffb0903cefeee93b6ff328")
	DeployTransactionObj := DeployTransaction{
		ContractAddressSalt: contractAddressSalt,
		Class:               c,
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
	// fmt.Println(c.ClassHash())
	// temp, _ := new(felt.Felt).SetString("0x3ec215c6c9028ff671b46a2a9814970ea23ed3c4bcc3838c6d1dcbf395263c3")
	// fmt.Println("Contract Address ", temp)
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
	// Read json file, parse it and generate a class
	contractDefinition, err := ioutil.ReadFile("TestData/class_definition_declare.json")
	if err != nil {
		t.Fatalf("expected no error but got %s", err)
	}
	c, err := contract.GenerateClass(contractDefinition)
	if err != nil {
		t.Fatalf("expected no error but got %s", err)
	}

	senderAddress, _ := new(felt.Felt).SetString("0x39291faa79897de1fd6fb1a531d144daa1590d058358171b83eadb3ceafed8")
	maxFee, _ := new(felt.Felt).SetString("0xf6dbd653833")
	nonce, _ := new(felt.Felt).SetString("0x5")
	signature1, _ := new(felt.Felt).SetString("0x221b9576c4f7b46d900a331d89146dbb95a7b03d2eb86b4cdcf11331e4df7f2")
	signature2, _ := new(felt.Felt).SetString("0x667d8062f3574ba9b4965871eec1444f80dacfa7114e1d9c74662f5672c0620")

	version, _ := new(felt.Felt).SetString("0x1")
	DeclareTransactionObj := DeclareTransaction{
		Class:         c,
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

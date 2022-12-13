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
	contractDefinition, err := ioutil.ReadFile("TestData/class_definition.json")
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

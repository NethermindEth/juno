// package feeder_test
package feeder_test

// NOTE: feederfakes creates an import cycle so testing has to be in a
// different package.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/pkg/feeder"

	"github.com/NethermindEth/juno/pkg/feeder/feederfakes"
	"github.com/bxcodec/faker"
	"github.com/stretchr/testify/assert"
)

var (
	httpClient = &feederfakes.FakeHttpClient{}
	client     *feeder.Client
)

func init() {
	var p feeder.HttpClient
	p = httpClient
	client = feeder.NewClient("https:/local", "/feeder_gateway/", &p)
}

func generateResponse(body string) *http.Response {
	return &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          ioutil.NopCloser(bytes.NewBufferString(body)),
		ContentLength: int64(len(body)),
		Header:        make(http.Header, 0),
	}
}

func StructFaker(a interface{}) (string, error) {
	s := reflect.ValueOf(a)
	err := faker.FakeData(&s)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func TestClient(t *testing.T) {
	_ = feeder.NewClient("https:/local", "/feeder_gateway/", nil)
}

func TestGetContractAddress(t *testing.T) {
	// XXX: Use raw string literal to formal JSON.
	body := "{\"GpsStatementVerifier\":\"0x47312450B3Ac8b5b8e247a6bB6d523e7605bDb60\",\"Starknet\":\"0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4\"}\n"
	httpClient.DoReturns(generateResponse(body), nil)
	var cOrig feeder.ContractAddresses
	err := json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	contractAddresses, err := client.GetContractAddresses()
	if err != nil {
		t.Fatal()
		return
	}
	assert.Equal(t, &cOrig, contractAddresses, "Contract Address does not match")
}

func TestCallContract(t *testing.T) {
	a := make(map[string][]string)
	body, err := json.Marshal(a)

	httpClient.DoReturns(generateResponse(string(body)), nil)
	contractResponse, err := client.CallContract(feeder.InvokeFunction{}, "", "latest")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &a, contractResponse, "CallContract response does not match")
}

func TestGetBlock(t *testing.T) {
	a := feeder.StarknetBlock{}
	body, err := StructFaker(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(body), nil)
	starknetBlock, err := client.GetBlock("", "latest")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &a, starknetBlock, "StarknetBlock does not match")
}

func TestGetStateUpdate(t *testing.T) {
	// XXX: Use raw string literal to formal JSON.
	body := "{\"block_hash\": \"0x6c9a1403d0d573ff7ce46b5ac0fba02b289bed60e26cacbc08f335e0a75fbbe\", \"new_root\": \"070d8a4d9843d6fb1a2b0564334133fc00a70b0ec9b9ccd791489c1c17a4a963\", \"old_root\": \"0025b10263a0ce4f984d313e8df3fae4dd473e6164b0c5e261a9af35892eafed\", \"state_diff\": {\"storage_diffs\": {\"0x5e4cac746d8709c776e901bcd5d23b2c2f696a0c850d47738bcbbddb1845ce5\": [], \"0x7091a8bbcb0f313b54af52c20f9b159258dca539d3caf0f7f007aae9b2252a9\": [{\"key\": \"0xf920571b9f85bdd92a867cfdc73319d0f8836f0e69e06e4c5566b6203f75cc\", \"value\": \"0x90aa7a9203bff78bfb24f0753c180a33d4bad95b1f4f510b36b00993815704\"}, {\"key\": \"0x1ccc09c8a19948e048de7add6929589945e25f22059c7345aaf7837188d8d05\", \"value\": \"0x1275efe83de1d12dce4bf30c44833afc7922b98f5cf9bcccc7cf941f4a3d814\"}], \"0x7d75ba4ac1ba55e1592375486d7937c1c55aab8fc3215de1429824d1c77da7a\": [{\"key\": \"0xf920571b9f85bdd92a867cfdc73319d0f8836f0e69e06e4c5566b6203f75cc\", \"value\": \"0x90aa7a9203bff78bfb24f0753c180a33d4bad95b1f4f510b36b00993815704\"}, {\"key\": \"0x1ccc09c8a19948e048de7add6929589945e25f22059c7345aaf7837188d8d05\", \"value\": \"0x5fcde101109bfe594ea7d47ec511213bf1a74d68840a1b36db7b649821a0e69\"}], \"0x4cf0f19cf486a762ea864077d368d8215f7d220b784a3360da52312e759ca66\": [{\"key\": \"0x37501df619c4fc4e96f6c0243f55e3abe7d1aca7db9af8f3740ba3696b3fdac\", \"value\": \"0x9\"}], \"0xec9b5d89811008b9a87145fd95f88827fbf0b8c273a7a7a51ad3523cdf9f55\": [{\"key\": \"0x5\", \"value\": \"0x0\"}, {\"key\": \"0x31d93a9cf0c6f9756c8323917128dd1fdf100f5fc3148f652204056ba88e26e\", \"value\": \"0x7e5\"}], \"0x7394cbe418daa16e42b87ba67372d4ab4a5df0b05c6e554d158458ce245bc10\": [{\"key\": \"0x7c68b6f31d543ce2476fc12f2411efd82f69d7d591505b7837ecff577125d87\", \"value\": \"0xa2a15d09519be00000\"}, {\"key\": \"0x1557182e4359a1f0c6301278e8f5b35a776ab58d39892581e357578fb287836\", \"value\": \"0x78118d2063b6ace88899ed8e4cfb5f3f\"}], \"0x741a8ac043f744786b0a61a7cb29238b5fba637484348a2ff30e9b1276ba41f\": [{\"key\": \"0x31d93a9cf0c6f9756c8323917128dd1fdf100f5fc3148f652204056ba88e26e\", \"value\": \"0x7c7\"}], \"0x11d0bd2a59aec732a27c3532decaa00db297c9d832f32eeefd35d27285302f2\": [{\"key\": \"0x5\", \"value\": \"0x64\"}]}, \"deployed_contracts\": [{\"address\": \"0x5e4cac746d8709c776e901bcd5d23b2c2f696a0c850d47738bcbbddb1845ce5\", \"contract_hash\": \"02864c45bd4ba3e66d8f7855adcadf07205c88f43806ffca664f1f624765207e\"}, {\"address\": \"0x7091a8bbcb0f313b54af52c20f9b159258dca539d3caf0f7f007aae9b2252a9\", \"contract_hash\": \"071c3c99f5cf76fc19945d4b8b7d34c7c5528f22730d56192b50c6bbfd338a64\"}, {\"address\": \"0x7d75ba4ac1ba55e1592375486d7937c1c55aab8fc3215de1429824d1c77da7a\", \"contract_hash\": \"071c3c99f5cf76fc19945d4b8b7d34c7c5528f22730d56192b50c6bbfd338a64\"}]}}\n"
	httpClient.DoReturns(generateResponse(body), nil)
	var cOrig feeder.StateUpdateResponse
	err := json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	getStateUpdate, err := client.GetStateUpdate("hash", "")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, getStateUpdate, "State Update response does not match")
}

func TestGetStateUpdateGoerli(t *testing.T) {
	// XXX: Use raw string literal to formal JSON.
	body := "{\"block_hash\": \"0x6c9a1403d0d573ff7ce46b5ac0fba02b289bed60e26cacbc08f335e0a75fbbe\", \"new_root\": \"070d8a4d9843d6fb1a2b0564334133fc00a70b0ec9b9ccd791489c1c17a4a963\", \"old_root\": \"0025b10263a0ce4f984d313e8df3fae4dd473e6164b0c5e261a9af35892eafed\", \"state_diff\": {\"storage_diffs\": {\"0x5e4cac746d8709c776e901bcd5d23b2c2f696a0c850d47738bcbbddb1845ce5\": [], \"0x7091a8bbcb0f313b54af52c20f9b159258dca539d3caf0f7f007aae9b2252a9\": [{\"key\": \"0xf920571b9f85bdd92a867cfdc73319d0f8836f0e69e06e4c5566b6203f75cc\", \"value\": \"0x90aa7a9203bff78bfb24f0753c180a33d4bad95b1f4f510b36b00993815704\"}, {\"key\": \"0x1ccc09c8a19948e048de7add6929589945e25f22059c7345aaf7837188d8d05\", \"value\": \"0x1275efe83de1d12dce4bf30c44833afc7922b98f5cf9bcccc7cf941f4a3d814\"}], \"0x7d75ba4ac1ba55e1592375486d7937c1c55aab8fc3215de1429824d1c77da7a\": [{\"key\": \"0xf920571b9f85bdd92a867cfdc73319d0f8836f0e69e06e4c5566b6203f75cc\", \"value\": \"0x90aa7a9203bff78bfb24f0753c180a33d4bad95b1f4f510b36b00993815704\"}, {\"key\": \"0x1ccc09c8a19948e048de7add6929589945e25f22059c7345aaf7837188d8d05\", \"value\": \"0x5fcde101109bfe594ea7d47ec511213bf1a74d68840a1b36db7b649821a0e69\"}], \"0x4cf0f19cf486a762ea864077d368d8215f7d220b784a3360da52312e759ca66\": [{\"key\": \"0x37501df619c4fc4e96f6c0243f55e3abe7d1aca7db9af8f3740ba3696b3fdac\", \"value\": \"0x9\"}], \"0xec9b5d89811008b9a87145fd95f88827fbf0b8c273a7a7a51ad3523cdf9f55\": [{\"key\": \"0x5\", \"value\": \"0x0\"}, {\"key\": \"0x31d93a9cf0c6f9756c8323917128dd1fdf100f5fc3148f652204056ba88e26e\", \"value\": \"0x7e5\"}], \"0x7394cbe418daa16e42b87ba67372d4ab4a5df0b05c6e554d158458ce245bc10\": [{\"key\": \"0x7c68b6f31d543ce2476fc12f2411efd82f69d7d591505b7837ecff577125d87\", \"value\": \"0xa2a15d09519be00000\"}, {\"key\": \"0x1557182e4359a1f0c6301278e8f5b35a776ab58d39892581e357578fb287836\", \"value\": \"0x78118d2063b6ace88899ed8e4cfb5f3f\"}], \"0x741a8ac043f744786b0a61a7cb29238b5fba637484348a2ff30e9b1276ba41f\": [{\"key\": \"0x31d93a9cf0c6f9756c8323917128dd1fdf100f5fc3148f652204056ba88e26e\", \"value\": \"0x7c7\"}], \"0x11d0bd2a59aec732a27c3532decaa00db297c9d832f32eeefd35d27285302f2\": [{\"key\": \"0x5\", \"value\": \"0x64\"}]}, \"deployed_contracts\": [{\"address\": \"0x5e4cac746d8709c776e901bcd5d23b2c2f696a0c850d47738bcbbddb1845ce5\", \"class_hash\": \"02864c45bd4ba3e66d8f7855adcadf07205c88f43806ffca664f1f624765207e\"}, {\"address\": \"0x7091a8bbcb0f313b54af52c20f9b159258dca539d3caf0f7f007aae9b2252a9\", \"class_hash\": \"071c3c99f5cf76fc19945d4b8b7d34c7c5528f22730d56192b50c6bbfd338a64\"}, {\"address\": \"0x7d75ba4ac1ba55e1592375486d7937c1c55aab8fc3215de1429824d1c77da7a\", \"class_hash\": \"071c3c99f5cf76fc19945d4b8b7d34c7c5528f22730d56192b50c6bbfd338a64\"}]}}\n"
	httpClient.DoReturns(generateResponse(body), nil)
	var cOrig feeder.StateUpdateResponseGoerli
	err := json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	getStateUpdate, err := client.GetStateUpdateGoerli("hash", "")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, stateUpdateResponseToGoerli(cOrig), getStateUpdate, "State Update response don't match")
}

func stateUpdateResponseToGoerli(res feeder.StateUpdateResponseGoerli) *feeder.StateUpdateResponse {
	deployedContracts := make([]feeder.DeployedContract, 0)

	for _, d := range res.StateDiff.DeployedContracts {
		deployedContracts = append(deployedContracts, feeder.DeployedContract{
			Address:      d.Address,
			ContractHash: d.ContractHash,
		})
	}
	return &feeder.StateUpdateResponse{
		BlockHash: res.BlockHash,
		NewRoot:   res.NewRoot,
		OldRoot:   res.OldRoot,
		StateDiff: feeder.StateDiff{
			DeployedContracts: deployedContracts,
			StorageDiffs:      res.StateDiff.StorageDiffs,
		},
	}
}

func TestGetFullContract(t *testing.T) {
	body := "{\"block_hash\": \"0x03a0ae1aaefeed60bafd6990f06d0b68fb593b5d9395ff726868ee61a6e1beb3\", \"block_number\": \"3\"}\n"
	httpClient.DoReturns(generateResponse(body), nil)
	var cOrig map[string]interface{}
	err := json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	getStateUpdate, err := client.GetFullContract("address", "hash", "number")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, cOrig, getStateUpdate, "GetFullContract response does not match")
}

func TestGetCode(t *testing.T) {
	a := feeder.CodeInfo{}
	body, err := StructFaker(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(body), nil)
	getCode, err := client.GetCode("hash", "", "latest")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &a, getCode, "GetCode response does not match")
}

func TestGetCode_ABICoverage(t *testing.T) {
	a := feederfakes.ReturnAbiInfo_Full()
	assert.Equal(t, "Struct-custom", a.Structs[0].Name)
}

func TestGetCode_FailType(t *testing.T) {
	a := feederfakes.ReturnAbiInfo_Fail()
	err := fmt.Errorf("unexpected type %s", "unknown")
	assert.Equal(t, err, a)
}

func TestGetTransaction(t *testing.T) {
	a := feeder.TransactionInfo{}
	err := faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	var cOrig feeder.TransactionInfo
	err = json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	transactionInfo, err := client.GetTransaction("", "id")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, transactionInfo, "GetTransaction response does not match")
}

func TestGetTransactionbyHash(t *testing.T) {
	a := feeder.TransactionInfo{}
	err := faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	var cOrig feeder.TransactionInfo
	err = json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	transactionInfo, err := client.GetTransaction("hash", "id")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, transactionInfo, "GetTransaction response does not match")
}

func TestGetTransactionReceipt(t *testing.T) {
	a := feeder.TransactionReceipt{}
	err := faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	var cOrig feeder.TransactionReceipt
	err = json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	transactionReceipt, err := client.GetTransactionReceipt("", "id")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, transactionReceipt, "GetTransactionReceipt response does not match")
}

func TestGetTransactionStatus(t *testing.T) {
	a := feeder.TransactionStatus{}
	err := faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	var cOrig feeder.TransactionStatus
	err = json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	transactionStatus, err := client.GetTransactionStatus("", "id")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, transactionStatus, "GetTransactionStatus response does not match")
}

func TestGetTransactionTrace(t *testing.T) {
	a := feeder.TransactionTrace{}
	err := faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	var cOrig feeder.TransactionTrace
	err = json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	transactionTrace, err := client.GetTransactionTrace("", "id")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, transactionTrace, "GetTransactionTrace response does not match")
}

func TestGetBlockHashById(t *testing.T) {
	body := "\"hash\"\n"
	httpClient.DoReturns(generateResponse(body), nil)
	var cOrig string
	err := json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	blockHash, err := client.GetBlockHashById("id")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, blockHash, "GetBlockHashById response does not match")
}

func TestGetBlockIdByHash(t *testing.T) {
	body := "\"id\"\n"
	httpClient.DoReturns(generateResponse(body), nil)
	var cOrig string
	err := json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		fmt.Println(err)
		t.Fatal()
	}
	var blockId *string
	blockId, err = client.GetBlockIDByHash("hash")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, blockId, "GetBlockIdByHash response does not match")
}

func TestGetTransactionHashById(t *testing.T) {
	body := "\"hash\"\n"
	httpClient.DoReturns(generateResponse(body), nil)
	var cOrig string
	err := json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	transactionHash, err := client.GetTransactionHashByID("hash")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, transactionHash, "GetTransactionHashById response does not match")
}

func TestGetTransactionIdByHash(t *testing.T) {
	body := "\"hash\"\n"
	httpClient.DoReturns(generateResponse(body), nil)
	var cOrig string
	err := json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	transactionId, err := client.GetTransactionIDByHash("hash")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, transactionId, "GetTransactionIdByHash response does not match")
}

func TestGetStorageAt(t *testing.T) {
	var body feeder.StorageInfo
	body = "\"storage\"\n"
	httpClient.DoReturns(generateResponse(string(body)), nil)
	var cOrig feeder.StorageInfo
	err := json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	transactionId, err := client.GetStorageAt("address", "key", "hash", "")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, transactionId, "GetStorageAt response does not match")
}

func TestEstimateTransactionFee(t *testing.T) {
	a := feeder.EstimateFeeResponse{}
	err := faker.FakeData(&a)
	if err != nil {
		t.Fatal()
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal()
	}
	httpClient.DoReturns(generateResponse(string(body)), nil)
	var cOrig feeder.EstimateFeeResponse
	err = json.Unmarshal([]byte(body), &cOrig)
	if err != nil {
		t.Fatal()
	}
	transactionFee, err := client.EstimateTransactionFee("contract", "func_name", "calldata", "signature")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, &cOrig, transactionFee, "GetTransactionTrace response does not match")
}

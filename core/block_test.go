package core

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

func hexToFelt(hex string) *felt.Felt {
	f, _ := new(felt.Felt).SetString(hex)
	return f
}

func TestBlockHash(t *testing.T) {
	uintToFelt := func(uint uint) *felt.Felt {
		f := new(felt.Felt).SetUint64(uint64(uint))
		return f
	}

	tests := []struct {
		block   *Block
		chain   utils.Network
		name    string
		want    string
		wantErr bool
	}{
		{
			// block 231579: goerli
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
			"goerli network (post 0.7.0 with sequencer address)",
			"0x40ffdbd9abbc4fc64652c50db94a29bce65c183316f304a95df624de708e746",
			false,
		},
		{
			// block 156000: goerli
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
			"goerli network (post 0.7.0 without sequencer address)",
			"0x1288267b119adefd52795c3421f8fabba78f49e911f39c1fb2f4e5eb8fb771",
			false,
		},
		{
			// block 1: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=1",
			&Block{
				hexToFelt("0x7d328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b"),
				1,
				hexToFelt("0x3f04ffa63e188d602796505a2ee4f6e1f294ee29a914b057af8e75b17259d9f"),
				nil,
				uintToFelt(1636989916),
				uintToFelt(4),
				hexToFelt("0x18bb7d6c1c558aa0a025f08a7d723a44b13008ffb444c432077f319a7f4897c"),
				uintToFelt(0),
				hexToFelt("0x0"),
				uintToFelt(0),
				hexToFelt(""),
			},
			0,
			"goerli network (pre 0.7.0 without sequencer address)",
			"0x75e00250d4343326f322e370df4c9c73c7be105ad9f532eeb97891a34d9e4a5",
			false,
		},
		{
			// block 16789: mainnet
			// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=16789"
			&Block{
				hexToFelt("0x3a97d46093a823719ac0c905e6548cebcbd6028b39f3cd184b0bf47498c1f66"),
				16789,
				hexToFelt("0x23710fe6dcc2fd95b74f66b30695e7b48506a17e5795676035c845fef50678c"),
				hexToFelt("0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9"),
				uintToFelt(1671087773),
				uintToFelt(214),
				hexToFelt("0x580a06bfc8c3fe39bbb7c5d16298b8928bf7c28f4c31b8e6b48fc25cd644fc1"),
				uintToFelt(962),
				hexToFelt("0x6f499789aabb31935810ce89d6ea9e9d37c5921c0d7fae2bd68f2fff5b7b93f"),
				hexToFelt("0x1"),
				hexToFelt(""),
			},
			1,
			"mainnet (post 0.7.0 with sequencer address)",
			"0x157b9e756f15e002e63580dddb8c8e342b9336c6d69a8cd6dc8eb8a75644040",
			false,
		},
		{
			// block 1: integration
			// "https://external.integration.starknet.io/feeder_gateway/get_block?blockNumber=1"
			&Block{
				hexToFelt("0x3ae41b0f023e53151b0c8ab8b9caafb7005d5f41c9ab260276d5bdc49726279"),
				1,
				hexToFelt("0x074abfb3f55d3f9c3967014e1a5ec7205949130ff8912dba0565daf70299144c"),
				nil,
				uintToFelt(1638978017),
				uintToFelt(4),
				hexToFelt("0xbf11745df434cbd284e13ca36354139a4bca2f6722e737c6136590990c8619"),
				uintToFelt(0),
				hexToFelt("0x0"),
				uintToFelt(0),
				hexToFelt(""),
			},
			3,
			"integration network (pre 0.7.0 without sequencer address)",
			"0x34e815552e42c5eb5233b99de2d3d7fd396e575df2719bf98e7ed2794494f86",
			true,
		},
		{
			// block 119802: goerli
			// https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=119802
			&Block{
				hexToFelt("0x3947adfc82697eaff29275eb4dba13c8e9d606d24246507d9c2faf8321f3c6b"),
				119802,
				hexToFelt("0x12c1e72707cd8a1226728aa8dee7fe70d281b482da5997c13db7c8746f9e8c0"),
				nil,
				uintToFelt(1647251113),
				uintToFelt(24),
				hexToFelt("0x3d31908e135bac6a6cea1eba760e845ba8e78b4970a5f7265b7792fb5a19470"),
				uintToFelt(27),
				hexToFelt("0x2016910f3a2fd5d241fde8c15c44a7cd0eafe6cdacb903822bd587c28e910b8"),
				uintToFelt(0),
				hexToFelt(""),
			},
			0,
			"goerli network (post 0.7.0 without sequencer address)",
			"0x62483d7a29a2aae440c4418e5ddf5acdbacc391af959d681e2dc9441b2895b6",
			true,
		},
		{
			// block 10: goerli2
			// https://alpha4-2.starknet.io/feeder_gateway/get_block?blockNumber=10
			&Block{
				hexToFelt("0x57467bd9f04b75e138357376d1f705604e0044fd677f7c12bbdfb9819d31b51"),
				10,
				hexToFelt("0x0097a5aa9bef614afc2f5f2b7fa1849f384be4bcc4e987b97e7640254eef0d7c"),
				hexToFelt("0x46a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b"),
				uintToFelt(1666883141),
				uintToFelt(1),
				hexToFelt("0x66dba4a3fa3b67af8f0469e58a064b837728f33328331b242c21bce5af03140"),
				uintToFelt(1),
				hexToFelt("0x160e8a530c118d3266447d46d29c7e9263ee59cf2da494d8339b0af9aae9427"),
				uintToFelt(1),
				hexToFelt(""),
			},
			2,
			"goerli2 network (post 0.7.0 with sequencer address)",
			"0x6902dad2e7ad976c59e032825b43474097396ad4a323d3782ede467540085f5",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.block.Hash(tt.chain)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, want error %v", err, tt.wantErr)
			}
			if !tt.wantErr && ("0x"+(got.Text(16)) != tt.want) {
				t.Errorf("got %s, want %s", "0x"+got.Text(16), tt.want)
			}
		})
	}
}

var (
	//go:embed testdata/block_156000.json
	block156000 []byte
	//go:embed testdata/block_1.json
	block1Goerli []byte
	//go:embed testdata/block_1_integration.json
	block1Integration []byte
	//go:embed testdata/block_16789_main.json
	blocks16789Main []byte
)

var receipts [][]*TransactionReceipt

func getTransactionReceipts(t *testing.T) {
	// https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=156000
	var blck156000 map[string]interface{}
	if err := json.Unmarshal(block156000, &blck156000); err != nil {
		t.Fatal(err)
	}
	receiptsInterface := blck156000["transaction_receipts"].([]interface{})
	txns := blck156000["transactions"].([]interface{})
	receipt156000 := generateReceipt(t, txns, receiptsInterface)

	// https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=1
	var blck1Goerli map[string]interface{}
	if err := json.Unmarshal(block1Goerli, &blck1Goerli); err != nil {
		t.Fatal(err)
	}
	receiptsInterface = blck1Goerli["transaction_receipts"].([]interface{})
	txns = blck1Goerli["transactions"].([]interface{})
	receipt1Goerli := generateReceipt(t, txns, receiptsInterface)

	// https://external.integration.starknet.io/feeder_gateway/get_block?blockNumber=1
	var blck1Integration map[string]interface{}
	if err := json.Unmarshal(block1Integration, &blck1Integration); err != nil {
		t.Fatal(err)
	}
	receiptsInterface = blck1Integration["transaction_receipts"].([]interface{})
	txns = blck1Integration["transactions"].([]interface{})
	receipt1Integration := generateReceipt(t, txns, receiptsInterface)

	// https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=16789
	var blck16789Main map[string]interface{}
	if err := json.Unmarshal(blocks16789Main, &blck16789Main); err != nil {
		t.Fatal(err)
	}
	receiptsInterface = blck16789Main["transaction_receipts"].([]interface{})
	txns = blck16789Main["transactions"].([]interface{})
	receipt16789Main := generateReceipt(t, txns, receiptsInterface)

	receipts = [][]*TransactionReceipt{
		receipt156000,
		receipt1Goerli,
		receipt1Integration,
		receipt16789Main,
	}
}

func generateReceipt(t *testing.T, txns []interface{}, receiptsInterface []interface{}) []*TransactionReceipt {
	receipts := make([]*TransactionReceipt, len(txns))

	transactionType := func(t string) TransactionType {
		switch t {
		case "DECLARE":
			return Declare
		case "DEPLOY":
			return Deploy
		case "DEPLOY_ACCOUNT":
			return DeployAccount
		case "INVOKE_FUNCTION":
			return Invoke
		case "L1_HANDLER":
			return L1Handler
		default:
			return -1
		}
	}

	for i, r := range receiptsInterface {
		receipt := r.(map[string]interface{})
		txn := txns[i].(map[string]interface{})
		var events []*Event
		for _, e := range receipt["events"].([]interface{}) {
			event := e.(map[string]interface{})
			var data []*felt.Felt
			for _, d := range event["data"].([]interface{}) {
				data = append(data, hexToFelt(d.(string)))
			}
			var keys []*felt.Felt
			for _, k := range event["keys"].([]interface{}) {
				keys = append(keys, hexToFelt(k.(string)))
			}
			events = append(events, &Event{
				Data: data,
				From: hexToFelt(event["from_address"].(string)),
				Keys: keys,
			})
		}
		var signatures []*felt.Felt
		if txn["signature"] != nil {
			for _, s := range txn["signature"].([]interface{}) {
				signatures = append(signatures, hexToFelt(s.(string)))
			}
		}
		// Some of these values are set to nil since they are not required to calculate the commitment.
		transactionReceipt := TransactionReceipt{
			Events:          events,
			Signatures:      signatures,
			TransactionHash: hexToFelt(receipt["transaction_hash"].(string)),
			Type:            transactionType(txn["type"].(string)),
		}
		receipts[i] = &transactionReceipt
	}
	return receipts
}

func init() {
	var t *testing.T
	getTransactionReceipts(t)
}

func assertCorrectCommitment(t *testing.T, got *felt.Felt, want string) {
	t.Helper()
	if "0x"+got.Text(16) != want {
		t.Errorf("got %s, want %s", "0x"+got.Text(16), want)
	}
}

func TestTransactionCommitment(t *testing.T) {
	tests := []struct {
		description string
		receipts    []*TransactionReceipt
		want        string
	}{
		{
			"receipt 1 (goerli)",
			receipts[0],
			"0x24638e0ca122d0260d54e901dc0942ea68bd1fc40a96b5da765985c47c92500",
		},
		{
			"receipt 2 (goerli)",
			receipts[1],
			"0x18bb7d6c1c558aa0a025f08a7d723a44b13008ffb444c432077f319a7f4897c",
		},
		{
			"receipt 1 (integration)",
			receipts[2],
			"0xbf11745df434cbd284e13ca36354139a4bca2f6722e737c6136590990c8619",
		},
		{
			"receipt 1 (mainnet)",
			receipts[3],
			"0x580a06bfc8c3fe39bbb7c5d16298b8928bf7c28f4c31b8e6b48fc25cd644fc1",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			commitment, _ := TransactionCommitment(test.receipts)
			assertCorrectCommitment(t, commitment, test.want)
		})
	}
}

func TestEventCommitment(t *testing.T) {
	tests := []struct {
		description string
		receipts    []*TransactionReceipt
		want        string
	}{
		{
			"receipt 1 (goerli)",
			receipts[0],
			"0x5d25e41d43b00681cc63ed4e13a82efe3e02f47e03173efbd737dd52ba88c7e",
		},
		{
			"receipt 2 (goerli)",
			receipts[1],
			"0x0",
		},
		{
			"receipt 3 (integration)",
			receipts[2],
			"0x0",
		},
		{
			"receipt 4 (mainnet)",
			receipts[3],
			"0x6f499789aabb31935810ce89d6ea9e9d37c5921c0d7fae2bd68f2fff5b7b93f",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			commitment, _, _ := EventData(test.receipts)
			assertCorrectCommitment(t, commitment, test.want)
		})
	}
}

package main

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/require"
)

func gzipped(t *testing.T, body []byte) []byte {
	t.Helper()
	compressed, err := gzipBytes(body)
	require.NoError(t, err)
	return compressed
}

func TestNewBlockInfo(t *testing.T) {
	tests := []struct {
		number          uint64
		wantTimestamp   uint64
		wantClassHashes []string
	}{
		{56377, 1712213818, []string{deprecatedHash}},
		{56378, 1712214058, []string{sierraHash, sierraHash}},
		{56379, 1712214299, []string{}},
	}
	for _, test := range tests {
		t.Run(mustResource(t, stateUpdate, blockKey{BlockNumber: test.number}).file, func(t *testing.T) {
			file := strconv.FormatUint(test.number, 10) + ".json"
			body := gzipped(t, readFixture(t, "state_update_with_block", file))

			info, err := newBlockInfo(body, test.number)
			require.NoError(t, err)
			require.Equal(t, blockInfo{
				number:      test.number,
				timestamp:   test.wantTimestamp,
				classHashes: test.wantClassHashes,
			}, info)
		})
	}
}

func TestNewBlockInfoRejects(t *testing.T) {
	lacksBlock := "get_state_update 5: state update response lacks block or state_update"
	tests := []struct {
		name    string
		body    []byte
		wantErr string
	}{
		{
			"not gzip",
			[]byte(`{"block": {}, "state_update": {}}`),
			"get_state_update 5: gzip: invalid header",
		},
		{"not json", gzipped(t, []byte("nope")), "get_state_update 5: invalid character"},
		{"missing block", gzipped(t, []byte(`{"state_update": {}}`)), lacksBlock},
		{"missing state update", gzipped(t, []byte(`{"block": {}}`)), lacksBlock},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := newBlockInfo(test.body, 5)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestClassHashes(t *testing.T) {
	var diff starknet.StateDiff
	require.NoError(t, json.Unmarshal([]byte(`{
		"deployed_contracts": [
			{"address": "0xa", "class_hash": "0x1"},
			{"address": "0xb", "class_hash": "0x2"}
		],
		"old_declared_contracts": ["0x3"],
		"declared_classes": [{"class_hash": "0x2", "compiled_class_hash": "0x4"}],
		"replaced_classes": [{"address": "0xc", "class_hash": "0x9"}]
	}`), &diff))
	require.Equal(t, []string{"0x1", "0x2", "0x3", "0x2"}, classHashes(&diff))
}

func TestUniqueClassHashes(t *testing.T) {
	tests := []struct {
		name   string
		blocks []blockInfo
		want   []string
	}{
		{"none", nil, nil},
		{"empty blocks", []blockInfo{{}, {}}, nil},
		{"sorted and deduplicated", []blockInfo{
			{classHashes: []string{"0xb", "0xa"}},
			{classHashes: []string{"0xa", "0xc", "0xa"}},
		}, []string{"0xa", "0xb", "0xc"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, uniqueClassHashes(test.blocks))
		})
	}
}

func TestIsSierra(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		want    bool
		wantErr string
	}{
		{"sierra", gzipped(t, readFixture(t, "class", sierraFixture+".json")), true, ""},
		{"deprecated", gzipped(t, readFixture(t, "class", deprecatedFixture+".json")), false, ""},
		{
			"not gzip",
			[]byte(`{"sierra_program": []}`),
			false,
			"get_class_by_hash 0x1: gzip: invalid header",
		},
		{"not json", gzipped(t, []byte("[")), false, "get_class_by_hash 0x1:"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := isSierra(test.body, "0x1")
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestCoreContractAddress(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		want    eth.Address
		wantErr string
	}{
		{"sepolia", gzipped(t, []byte(contractAddressesJSON)), networks.Sepolia.CoreContractAddress, ""},
		{"missing key", gzipped(t, []byte(`{}`)), eth.Address{}, ""},
		{
			"not gzip",
			[]byte(contractAddressesJSON),
			eth.Address{},
			"get_contract_addresses: gzip: invalid header",
		},
		{
			"bad address",
			gzipped(t, []byte(`{"Starknet": "0xzz"}`)),
			eth.Address{},
			"get_contract_addresses:",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := coreContractAddress(test.body)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestHexAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
	}{
		{
			"lowercases",
			"0xE2Bb56ee936fd6433DC0F6e7e3b8365C906AA057",
			"0xe2bb56ee936fd6433dc0f6e7e3b8365c906aa057",
		},
		{
			"keeps leading zeros",
			"0x0000000000000000000000000000000000000001",
			"0x0000000000000000000000000000000000000001",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, hexAddress(eth.AddressFromString(test.address)))
		})
	}
}

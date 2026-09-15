package main

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResource(t *testing.T) {
	tests := []struct {
		name      string
		got       func() (resource, error)
		wantName  string
		wantFile  string
		wantQuery string
	}{
		{
			"contract addresses",
			func() (resource, error) { return contractAddresses.resource(struct{}{}) },
			"get_contract_addresses", "get_contract_addresses.json.gz", "",
		},
		{
			"block",
			func() (resource, error) { return block.resource(blockKey{BlockNumber: 5}) },
			"get_block", "get_block/5.json.gz", "blockNumber=5&headerOnly=true",
		},
		{
			"state update",
			func() (resource, error) { return stateUpdate.resource(blockKey{BlockNumber: 5}) },
			"get_state_update", "get_state_update/5.json.gz",
			"blockNumber=5&includeBlock=true&includeSignature=true",
		},
		{
			"class",
			func() (resource, error) { return classByHash.resource(classKey{ClassHash: "0xabc"}) },
			"get_class_by_hash", "get_class_by_hash/0xabc.json.gz", "blockNumber=latest&classHash=0xabc",
		},
		{
			"compiled class",
			func() (resource, error) { return compiledClass.resource(classKey{ClassHash: "0xabc"}) },
			"get_compiled_class_by_class_hash", "get_compiled_class_by_class_hash/0xabc.json.gz",
			"blockNumber=latest&classHash=0xabc",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.got()
			require.NoError(t, err)
			require.Equal(t, test.wantName, got.name)
			require.Equal(t, test.wantFile, got.file)
			require.Equal(t, test.wantQuery, got.query.Encode())
		})
	}
}

func TestResourceURL(t *testing.T) {
	feederURL, err := url.Parse("http://feeder.example/feeder_gateway/")
	require.NoError(t, err)

	tests := []struct {
		name     string
		resource resource
		want     string
	}{
		{
			"block",
			mustResource(t, block, blockKey{BlockNumber: 5}),
			"http://feeder.example/feeder_gateway/get_block?blockNumber=5&headerOnly=true",
		},
		{
			"contract addresses",
			mustResource(t, contractAddresses, struct{}{}),
			"http://feeder.example/feeder_gateway/get_contract_addresses",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, test.resource.url(feederURL).String())
		})
	}
}

func TestCheckFixed(t *testing.T) {
	tests := []struct {
		name    string
		check   func(url.Values) error
		query   string
		wantErr string
	}{
		{"block header only", block.checkFixed, "blockNumber=1&headerOnly=true", ""},
		{
			"block missing header only",
			block.checkFixed,
			"blockNumber=1",
			`get_block: unsupported query "blockNumber=1"`,
		},
		{"block full body", block.checkFixed, "headerOnly=false", "get_block: unsupported query"},
		{"block bad bool", block.checkFixed, "headerOnly=maybe", "get_block:"},
		{"state update both", stateUpdate.checkFixed, "includeBlock=true&includeSignature=true", ""},
		{
			"state update without signature",
			stateUpdate.checkFixed,
			"includeBlock=true",
			"get_state_update: unsupported query",
		},
		{"class at latest", classByHash.checkFixed, "classHash=0x1&blockNumber=latest", ""},
		{
			"class at number",
			classByHash.checkFixed,
			"classHash=0x1&blockNumber=5",
			"get_class_by_hash: unsupported query",
		},
		{
			"class without block",
			classByHash.checkFixed,
			"classHash=0x1",
			"get_class_by_hash: unsupported query",
		},
		{"compiled class at latest", compiledClass.checkFixed, "blockNumber=latest", ""},
		{"contract addresses ignores extras", contractAddresses.checkFixed, "anything=1", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values, err := url.ParseQuery(test.query)
			require.NoError(t, err)

			err = test.check(values)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestKey(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    blockKey
		wantErr string
	}{
		{"number", "blockNumber=7&headerOnly=true", blockKey{BlockNumber: 7}, ""},
		{"not a number", "blockNumber=seven", blockKey{}, "get_block:"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values, err := url.ParseQuery(test.query)
			require.NoError(t, err)

			got, err := block.key(values)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

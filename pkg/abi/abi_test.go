package abi

import (
	"embed"
	"encoding/json"
	"testing"
)

//go:embed test_assets/*
var testAssets embed.FS

func loadABIPaths() ([]string, error) {
	items, err := testAssets.ReadDir("test_assets")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, item := range items {
		if !item.IsDir() {
			paths = append(paths, "test_assets/"+item.Name())
		}
	}
	return paths, nil
}

func TestUnmarshalJSON(t *testing.T) {
	paths, err := loadABIPaths()
	if err != nil {
		t.Error(err)
	}
	var tests []struct {
		Data []byte
		Err  bool
	}
	// Generate tests with ABI files
	for _, p := range paths {
		rawData, err := testAssets.ReadFile(p)
		if err != nil {
			t.Error(err)
		}
		var testItem struct {
			Contract string
			Abi      json.RawMessage
		}
		if err := json.Unmarshal(rawData, &testItem); err != nil {
			t.Error(err)
		}
		tests = append(tests, struct {
			Data []byte
			Err  bool
		}{
			Data: testItem.Abi,
			Err:  false,
		})
	}
	// Add test with the empty ABI
	tests = append(tests, []struct {
		Data []byte
		Err  bool
	}{
		{
			Data: []byte("[]"),
			Err:  false,
		},
		{
			Data: []byte("[{\"type\":\"invalidType\"}]"),
			Err:  true,
		},
	}...)
	for _, test := range tests {
		abi := new(Abi)
		err := json.Unmarshal(test.Data, abi)
		if err != nil && !test.Err {
			t.Errorf("unexpected error: %s", err.Error())
		}
		if err == nil && test.Err {
			t.Error("expected error")
		}
	}
}

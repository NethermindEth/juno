package contract

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
)

func TestGenerateClass(t *testing.T) {
	// Read json file, parse it and generate a class
	contractDefinition, err := os.ReadFile("contract_definition.json")
	if err != nil {
		t.Fatalf("expected no error but got %s", err)
	}

	t.Run("GenerateClass", func(t *testing.T) {
		_, err := GenerateClass(contractDefinition)
		if err != nil {
			t.Fatalf("expected no error but got %s", err)
		}
	})
}

func TestClassHash(t *testing.T) {
	tests := []struct {
		want string
		file string
	}{
		{
			want: "0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118",
			file: "contract_definition.json",
		},
		{
			want: "0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
			file: "genesis_contract.json",
		},
	}
	for _, tt := range tests {
		// Read json file, parse it and generate a class
		contractDefinition, err := ioutil.ReadFile(tt.file)
		if err != nil {
			t.Fatalf("expected no error but got %s", err)
		}
		c, err := GenerateClass(contractDefinition)
		if err != nil {
			t.Fatalf("expected no error but got %s", err)
		}
		t.Run("ClassHash", func(t *testing.T) {
			classHash, err := c.ClassHash()
			if err != nil {
				t.Fatalf("expected no error but got %s", err)
			}
			want, _ := new(felt.Felt).SetString(tt.want)
			if !classHash.Equal(want) {
				t.Errorf("ClassHash got %s, want %s", classHash.Text(16), want.Text(16))
			}
		})
	}
}

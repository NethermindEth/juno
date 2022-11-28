package contract

import (
	"io/ioutil"
	"testing"
)

func TestGenerateClass(t *testing.T) {
	// Read json file, parse it and generate a class
	contractDefinition, err := ioutil.ReadFile("contract_definition.json")
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
			want: "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918",
			file: "contract_definition.json",
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
			_, err := c.ClassHash()
			if err != nil {
				t.Fatalf("expected no error but got %s", err)
			}
			// want, _ := new(felt.Felt).SetString(tt.want)
			// if !classHash.Equal(want) {
			// 	t.Errorf("ClassHash got %s, want %s", classHash.Text(16), want.Text(16))
			// }
		})
	}
}

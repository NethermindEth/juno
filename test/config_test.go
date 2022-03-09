package test

import (
	"github.com/NethermindEth/juno/cmd/juno/cli"
	"github.com/NethermindEth/juno/internal/ospkg"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	cli.GenerateConfig()

	cfgPath := filepath.Join(ospkg.ConfigDir, "juno", "juno.yaml")

	contents, err := ioutil.ReadFile(cfgPath)

	if err != nil {
		t.Fatal(err)
	}

	data := cli.Config{}

	err2 := yaml.Unmarshal(contents, &data)

	if err2 != nil {
		t.Fatal(err2)
	}
}

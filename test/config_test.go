package test

import (
	"github.com/NethermindEth/juno/cmd/juno/cli"
	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	tmp := t.TempDir()
	err := cli.DefaultConfig(tmp)
	if err != nil {
		t.Fail()
	}
	dbPath := filepath.Join(tmp, cli.ProjectDir, cli.DbPath)
	original := cli.NewConfig(dbPath)

	finalPath := filepath.Join(tmp, cli.ProjectDir, cli.CfgFileName)

	contents, err := ioutil.ReadFile(finalPath)

	if err != nil {
		t.Fatal(err)
	}

	data := cli.Config{}

	err2 := yaml.Unmarshal(contents, &data)

	if err2 != nil {
		t.Fatal(err2)
	}
	if !cmp.Equal(&data, original) {
		t.Fail()
	}

}

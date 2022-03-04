package configs

import (
	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	tmp := t.TempDir()
	err := Generate(tmp)
	if err != nil {
		t.Fail()
	}
	dbPath := filepath.Join(tmp, ProjectFolderName, DatabasePath)
	original := NewConfiguration(dbPath)

	finalPath := filepath.Join(tmp, ProjectFolderName, ConfigFileName)

	contents, err := ioutil.ReadFile(finalPath)

	if err != nil {
		t.Fatal(err)
	}

	data := Configuration{}

	err2 := yaml.Unmarshal(contents, &data)

	if err2 != nil {
		t.Fatal(err2)
	}
	if !cmp.Equal(&data, original) {
		t.Fail()
	}

}

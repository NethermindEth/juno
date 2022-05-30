package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestConfig(t *testing.T) {
	New()
	f, err := os.ReadFile(filepath.Join(Dir, "juno.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg *Config
	err = yaml.Unmarshal(f, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !Exists() {
		t.Fatal("default config file must be exists")
	}
}

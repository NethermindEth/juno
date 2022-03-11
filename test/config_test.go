package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/internal/config"
	"gopkg.in/yaml.v2"
)

func TestConfig(t *testing.T) {
	config.New()
	f, err := os.ReadFile(filepath.Join(config.Dir, "juno.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg *config.Config
	err = yaml.Unmarshal(f, &cfg)
	if err != nil {
		t.Fatal(err)
	}
}

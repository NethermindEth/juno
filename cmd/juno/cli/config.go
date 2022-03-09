package cli

import (
	"github.com/NethermindEth/juno/internal/errpkg"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/ospkg"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

func GenerateConfig() {
	// Create default Config.
	cfgPath := filepath.Join(ospkg.ConfigDir, "juno", "juno.yaml")
	log.Default.With("Path", cfgPath).Info("Creating default Config.")

	// Create the juno configuration directory if it does not exist.
	cfgDirPath := filepath.Join(ospkg.ConfigDir, "juno")
	if _, err := os.Stat(cfgDirPath); os.IsNotExist(err) {
		err := os.MkdirAll(cfgDirPath, 0755)
		errpkg.CheckFatal(err, "Failed to create Config directory.")
	}

	data, err := yaml.Marshal(&Config{
		Rpc:    rpcConfig{Enabled: true, Port: 8080},
		DbPath: filepath.Join(ospkg.DataDir, "juno"),
	})
	errpkg.CheckFatal(err, "Failed to marshal Config instance to byte data.")
	err = os.WriteFile(cfgPath, data, 0644)
	errpkg.CheckFatal(err, "Failed to write Config file.")

}

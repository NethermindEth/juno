package main_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"syscall"
	"testing"
	"time"

	junoCmd "github.com/NethermindEth/juno/cmd/juno"
	"github.com/NethermindEth/juno/internal/juno"
	"github.com/NethermindEth/juno/internal/utils"
	"gotest.tools/v3/assert"
)

type spyJuno struct {
	cfg   *juno.Config
	calls []string
	exit  chan struct{}
}

func newSpyJuno(junoCfg *juno.Config) (juno.StarkNetNode, error) {
	return &spyJuno{cfg: junoCfg, exit: make(chan struct{})}, nil
}

func (s *spyJuno) Run() error {
	s.calls = append(s.calls, "run")
	<-s.exit
	return nil
}

func (s *spyJuno) Shutdown() error {
	s.calls = append(s.calls, "shutdown")
	s.exit <- struct{}{}
	return nil
}

func TestNewCmd(t *testing.T) {
	t.Run("greeting", func(t *testing.T) {
		expected := `
       _                    
      | |                   
      | |_   _ _ __   ___   
  _   | | | | | '_ \ / _ \  
 | |__| | |_| | | | | (_) |  
  \____/ \__,_|_| |_|\___/  

Juno is a Go implementation of a StarkNet full node client made with ❤️ by Nethermind.

`
		b := new(bytes.Buffer)

		cmd := junoCmd.NewCmd(newSpyJuno, quitTest())
		cmd.SetOut(b)
		err := cmd.Execute()
		assert.NilError(t, err)

		actual := b.String()
		assert.Equal(t, expected, actual)

		n, ok := junoCmd.StarkNetNode.(*spyJuno)
		assert.Equal(t, true, ok)
		assert.DeepEqual(t, []string{"run", "shutdown"}, n.calls)
	})

	t.Run("config precedence", func(t *testing.T) {
		// The purpose of these tests are to ensure the precedence of our config
		// values is respected. Since viper offers this feature, it would be
		// redundant to enumerate all combinations. Thus, only a select few are
		// tested for sanity. These tests are not intended to perform semantics
		// checks on the config, those will be checked by the StarkNetNode
		// implementation.
		defaultVerbosity := "info"
		defaultRpcPort := uint16(6060)
		defaultMetrics := false
		defaultDbPath := ""
		defaultNetwork := utils.GOERLI
		defaultEthNode := ""

		tests := map[string]struct {
			cfgFile         func(t *testing.T, cfg string) (string, func())
			cfgFileContents string
			expectErr       bool
			inputArgs       []string
			expectedConfig  *juno.Config
		}{
			"default config with no flags": {
				inputArgs: []string{""},
				expectedConfig: &juno.Config{
					Verbosity:    defaultVerbosity,
					RpcPort:      defaultRpcPort,
					Metrics:      defaultMetrics,
					DatabasePath: defaultDbPath,
					Network:      defaultNetwork, EthNode: defaultEthNode,
				},
			},
			"config file path is empty string": {
				inputArgs: []string{"--config", ""},
				expectedConfig: &juno.Config{
					Verbosity:    defaultVerbosity,
					RpcPort:      defaultRpcPort,
					Metrics:      defaultMetrics,
					DatabasePath: defaultDbPath,
					Network:      defaultNetwork, EthNode: defaultEthNode,
				},
			},
			"config file doesn't exist": {
				inputArgs: []string{"--config", "config-file-test.yaml"},
				expectErr: true,
			},
			"config file contents are empty": {
				cfgFile:         tempCfgFile,
				cfgFileContents: "\n",
				expectedConfig: &juno.Config{
					Verbosity: defaultVerbosity,
					RpcPort:   defaultRpcPort,
					Metrics:   defaultMetrics,
					Network:   defaultNetwork, EthNode: defaultEthNode,
				},
			},
			"config file with all settings but without any other flags": {
				cfgFile: tempCfgFile,
				cfgFileContents: `verbosity: "debug"
rpc-port: 4576
metrics: true
db-path: /home/.juno
network: 1
eth-node: "https://some-ethnode:5673"
`,
				expectedConfig: &juno.Config{
					Verbosity:    "debug",
					RpcPort:      4576,
					Metrics:      true,
					DatabasePath: "/home/.juno",
					Network:      utils.MAINNET,
					EthNode:      "https://some-ethnode:5673",
				},
			},
			"config file with some settings but without any other flags": {
				cfgFile: tempCfgFile,
				cfgFileContents: `verbosity: "debug"
rpc-port: 4576
metrics: true
`,
				expectedConfig: &juno.Config{
					Verbosity:    "debug",
					RpcPort:      4576,
					Metrics:      true,
					DatabasePath: defaultDbPath,
					Network:      defaultNetwork,
					EthNode:      defaultEthNode,
				},
			},
			"all flags without config file": {
				inputArgs: []string{
					"--verbosity", "debug", "--rpc-port", "4576",
					"--metrics", "--db-path", "/home/.juno", "--network", "1",
					"--eth-node", "https://some-ethnode:5673",
				},
				expectedConfig: &juno.Config{
					Verbosity:    "debug",
					RpcPort:      4576,
					Metrics:      true,
					DatabasePath: "/home/.juno",
					Network:      utils.MAINNET,
					EthNode:      "https://some-ethnode:5673",
				},
			},
			"some flags without config file": {
				inputArgs: []string{
					"--verbosity", "debug", "--rpc-port", "4576", "--db-path", "/home/.juno",
					"--network", "1",
				},
				expectedConfig: &juno.Config{
					Verbosity:    "debug",
					RpcPort:      4576,
					Metrics:      defaultMetrics,
					DatabasePath: "/home/.juno",
					Network:      utils.MAINNET,
					EthNode:      defaultEthNode,
				},
			},
			"all setting set in both config file and flags": {
				cfgFile: tempCfgFile,
				cfgFileContents: `verbosity: "debug"
rpc-port: 4576
metrics: true
db-path: /home/config-file/.juno
network: 0
eth-node: "https://some-ethnode:5673"
`,
				inputArgs: []string{
					"--verbosity", "error", "--rpc-port", "4577",
					"--metrics", "--db-path", "/home/flag/.juno", "--network", "1",
					"--eth-node", "https://some-ethnode:5674",
				},
				expectedConfig: &juno.Config{
					Verbosity:    "error",
					RpcPort:      4577,
					Metrics:      true,
					DatabasePath: "/home/flag/.juno",
					Network:      utils.MAINNET,
					EthNode:      "https://some-ethnode:5674",
				},
			},
			"some setting set in both config file and flags": {
				cfgFile: tempCfgFile,
				cfgFileContents: `verbosity: "panic"
rpc-port: 4576
network: 1
`,
				inputArgs: []string{
					"--metrics", "--db-path", "/home/flag/.juno", "--eth-node",
					"https://some-ethnode:5674",
				},
				expectedConfig: &juno.Config{
					Verbosity:    "panic",
					RpcPort:      4576,
					Metrics:      true,
					DatabasePath: "/home/flag/.juno",
					Network:      utils.MAINNET,
					EthNode:      "https://some-ethnode:5674",
				},
			},
			"some setting set in default, config file and flags": {
				cfgFile:         tempCfgFile,
				cfgFileContents: `network: 1`,
				inputArgs: []string{
					"--metrics", "--db-path", "/home/flag/.juno", "--eth-node",
					"https://some-ethnode:5674",
				},
				expectedConfig: &juno.Config{
					Verbosity:    defaultVerbosity,
					RpcPort:      defaultRpcPort,
					Metrics:      true,
					DatabasePath: "/home/flag/.juno",
					Network:      utils.MAINNET,
					EthNode:      "https://some-ethnode:5674",
				},
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				if tc.cfgFile != nil {
					fileN, cleanup := tc.cfgFile(t, tc.cfgFileContents)
					defer cleanup()
					tc.inputArgs = append(tc.inputArgs, []string{"--config", fileN}...)
				}

				cmd := junoCmd.NewCmd(newSpyJuno, quitTest())
				cmd.SetArgs(tc.inputArgs)

				err := cmd.Execute()
				if tc.expectErr {
					assert.Error(t, err, err.Error(), "error executing juno command")
					return
				}
				assert.NilError(t, err)

				n, ok := junoCmd.StarkNetNode.(*spyJuno)
				assert.Equal(t, true, ok)
				assert.DeepEqual(t, tc.expectedConfig, n.cfg)
				assert.DeepEqual(t, []string{"run", "shutdown"}, n.calls)
			})
		}
	})
}

func quitTest() chan os.Signal {
	exit := make(chan os.Signal, 1)
	go func() {
		time.Sleep(1 * time.Millisecond)
		exit <- syscall.SIGINT
	}()
	return exit
}

func tempCfgFile(t *testing.T, cfg string) (string, func()) {
	f, err := ioutil.TempFile("", "junoCfg.*.yaml")
	assert.NilError(t, err)

	defer func() {
		err = f.Close()
		assert.NilError(t, err)
	}()

	_, err = f.WriteString(cfg)
	assert.NilError(t, err)

	err = f.Sync()
	assert.NilError(t, err)

	return f.Name(), func() {
		err = os.Remove(f.Name())
		assert.NilError(t, err)
	}
}

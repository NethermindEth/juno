package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	dataFlag = "data"
	fromFlag = "from"
	toFlag   = "to"

	networkFlag           = "network"
	apiKeyFlag            = "api-key"
	concurrencyFlag       = "concurrency"
	captureTimeoutFlag    = "capture-timeout"
	captureRetriesFlag    = "capture-retries"
	captureRetryDelayFlag = "capture-retry-delay"

	listenFlag   = "listen"
	tipFlag      = "tip"
	intervalFlag = "interval"
	speedFlag    = "speed"
	latencyFlag  = "latency"

	logLevelFlag = "log-level"
)

const (
	defaultConcurrency       = 8
	defaultCaptureTimeout    = 30 * time.Second
	defaultCaptureRetries    = 5
	defaultCaptureRetryDelay = time.Second

	defaultListen   = "127.0.0.1:7070"
	listenOff       = "off"
	defaultInterval = 2 * time.Second
)

type config struct {
	data string
	from uint64
	to   uint64

	networkName       string
	network           *networks.Network
	apiKey            string
	concurrency       int
	captureTimeout    time.Duration
	captureRetries    int
	captureRetryDelay time.Duration

	listen   string
	tip      uint64
	interval time.Duration
	speed    float64
	latency  time.Duration

	logLevel *log.Level
}

func (config *config) register(command *cobra.Command) {
	flags := command.Flags()
	config.registerRange(flags)
	config.registerCapture(flags)
	config.registerServe(flags)
	config.logLevel = log.NewLevel(log.INFO)
	flags.Var(config.logLevel, logLevelFlag, "Log level: debug, info, warn or error.")
	command.MarkFlagsMutuallyExclusive(intervalFlag, speedFlag)
}

func (config *config) registerRange(flags *pflag.FlagSet) {
	flags.StringVar(&config.data, dataFlag, "", "Dataset directory. Use one directory per network.")
	flags.Uint64Var(&config.from, fromFlag, 0, "First block served.")
	flags.Uint64Var(&config.to, toFlag, 0, "Last block served, inclusive.")
}

func (config *config) registerCapture(flags *pflag.FlagSet) {
	flags.StringVar(
		&config.networkName,
		networkFlag,
		"",
		"Capture source: mainnet, sepolia or sepolia-integration. Absent means offline.",
	)
	flags.StringVar(&config.apiKey, apiKeyFlag, "", "X-Throttling-Bypass header sent during capture.")
	flags.IntVar(
		&config.concurrency,
		concurrencyFlag,
		defaultConcurrency,
		"Parallel capture requests.",
	)
	flags.DurationVar(
		&config.captureTimeout,
		captureTimeoutFlag,
		defaultCaptureTimeout,
		"Per-request timeout during capture.",
	)
	flags.IntVar(
		&config.captureRetries,
		captureRetriesFlag,
		defaultCaptureRetries,
		"Retries per request during capture.",
	)
	flags.DurationVar(
		&config.captureRetryDelay,
		captureRetryDelayFlag,
		defaultCaptureRetryDelay,
		"Delay between retries during capture.",
	)
}

func (config *config) registerServe(flags *pflag.FlagSet) {
	flags.StringVar(
		&config.listen,
		listenFlag,
		defaultListen,
		`Serve on host:port. ":7070" binds all interfaces, "off" captures only.`,
	)
	flags.Uint64Var(
		&config.tip,
		tipFlag,
		0,
		"Initial tip, in [from, to]. Defaults to from; --tip <to> serves everything at once.",
	)
	flags.DurationVar(
		&config.interval,
		intervalFlag,
		defaultInterval,
		"Advance the tip by one block every interval.",
	)
	flags.Float64Var(
		&config.speed,
		speedFlag,
		0,
		"Replay captured block timestamps at this multiplier instead of --interval.",
	)
	flags.DurationVar(&config.latency, latencyFlag, 0, "Fixed delay added to every response.")
}

func (config *config) validate(flags *pflag.FlagSet) error {
	if err := config.validateRange(flags); err != nil {
		return err
	}
	if err := config.validateCapture(); err != nil {
		return err
	}
	if err := config.validateTip(flags); err != nil {
		return err
	}
	if err := config.validatePacing(flags); err != nil {
		return err
	}
	return config.resolveMode()
}

func (config *config) validateRange(flags *pflag.FlagSet) error {
	var missing []string
	for _, name := range []string{dataFlag, fromFlag, toFlag} {
		if !flags.Changed(name) {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required flag(s) not set: %s", strings.Join(missing, ", "))
	}

	if config.from > config.to {
		return fmt.Errorf("--%s (%d) must be <= --%s (%d)", fromFlag, config.from, toFlag, config.to)
	}
	return nil
}

func (config *config) validateCapture() error {
	if config.concurrency < 1 {
		return fmt.Errorf("--%s must be >= 1", concurrencyFlag)
	}
	if config.captureTimeout <= 0 {
		return fmt.Errorf("--%s must be > 0", captureTimeoutFlag)
	}
	if config.captureRetries < 0 {
		return fmt.Errorf("--%s must be >= 0", captureRetriesFlag)
	}
	if config.captureRetryDelay < 0 {
		return fmt.Errorf("--%s must be >= 0", captureRetryDelayFlag)
	}
	return nil
}

func (config *config) validateTip(flags *pflag.FlagSet) error {
	if !flags.Changed(tipFlag) {
		config.tip = config.from
	}
	if config.tip < config.from || config.tip > config.to {
		return fmt.Errorf("--%s (%d) must be in [%d, %d]", tipFlag, config.tip, config.from, config.to)
	}
	return nil
}

func (config *config) validatePacing(flags *pflag.FlagSet) error {
	if flags.Changed(speedFlag) && config.speed <= 0 {
		return fmt.Errorf("--%s must be > 0", speedFlag)
	}
	if config.interval <= 0 {
		return fmt.Errorf("--%s must be > 0", intervalFlag)
	}
	if config.latency < 0 {
		return fmt.Errorf("--%s must be >= 0", latencyFlag)
	}
	return nil
}

func (config *config) resolveMode() error {
	if config.networkName != "" {
		var network networks.Network
		if err := network.Set(config.networkName); err != nil {
			return fmt.Errorf("--%s %q: %w", networkFlag, config.networkName, err)
		}
		config.network = &network
	}

	if config.listen == listenOff {
		config.listen = ""
	}
	if config.network == nil && config.listen == "" {
		return errors.New("nothing to do: set --network to capture, --listen to serve, or both")
	}
	return nil
}

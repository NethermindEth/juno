package main

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/stretchr/testify/require"
)

func parseConfig(t *testing.T, args ...string) (*config, error) {
	t.Helper()
	var parsed *config
	command := newCommand(func(_ context.Context, config *config) error {
		parsed = config
		return nil
	})
	command.SetArgs(args)
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	return parsed, command.ExecuteContext(t.Context())
}

func TestValidateRejects(t *testing.T) {
	required := []string{"--data", "d", "--from", "1", "--to", "3"}
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no flags", nil, "required flag(s) not set: --data, --from, --to"},
		{"missing to", []string{"--data", "d", "--from", "1"}, "required flag(s) not set: --to"},
		{
			"from above to",
			[]string{"--data", "d", "--from", "5", "--to", "4"},
			"--from (5) must be <= --to (4)",
		},
		{"tip below from", append(required, "--tip", "0"), "--tip (0) must be in [1, 3]"},
		{"tip above to", append(required, "--tip", "4"), "--tip (4) must be in [1, 3]"},
		{
			"interval and speed",
			append(required, "--interval", "1s", "--speed", "2"),
			"[interval speed] were all set",
		},
		{"zero speed", append(required, "--speed", "0"), "--speed must be > 0"},
		{"zero interval", append(required, "--interval", "0"), "--interval must be > 0"},
		{"negative latency", append(required, "--latency", "-1ms"), "--latency must be >= 0"},
		{"zero concurrency", append(required, "--concurrency", "0"), "--concurrency must be >= 1"},
		{
			"zero capture timeout",
			append(required, "--capture-timeout", "0"),
			"--capture-timeout must be > 0",
		},
		{
			"negative retries",
			append(required, "--capture-retries", "-1"),
			"--capture-retries must be >= 0",
		},
		{
			"negative retry delay",
			append(required, "--capture-retry-delay", "-1s"),
			"--capture-retry-delay must be >= 0",
		},
		{
			"unknown network",
			append(required, "--network", "goerli"),
			`--network "goerli": unknown network`,
		},
		{"nothing to do", append(required, "--listen", "off"), "nothing to do"},
		{"bad log level", append(required, "--log-level", "loud"), "invalid argument"},
		{
			"rpc url without scheme",
			append(required, "--rpc-url", "node:6060"),
			`--rpc-url "node:6060" must be an http or https URL`,
		},
		{
			"rpc url with other scheme",
			append(required, "--rpc-url", "ftp://node:6060"),
			`--rpc-url "ftp://node:6060" must be an http or https URL`,
		},
		{
			"rpc url without host",
			append(required, "--rpc-url", "http://"),
			`--rpc-url "http://" must be an http or https URL`,
		},
		{
			"unparsable rpc url",
			append(required, "--rpc-url", "://node"),
			`--rpc-url "://node": parse "://node": missing protocol scheme`,
		},
		{
			"preconfirmed capture without rpc url",
			append(required, "--network", "sepolia", "--preconfirmed"),
			"--preconfirmed with --network requires --rpc-url",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := parseConfig(t, test.args...)
			require.ErrorContains(t, err, test.wantErr)
			require.Nil(t, parsed)
		})
	}
}

func TestValidateResolves(t *testing.T) {
	required := []string{"--data", "d", "--from", "1", "--to", "3"}
	tests := []struct {
		name        string
		args        []string
		wantTip     uint64
		wantListen  string
		wantNetwork *networks.Network
		wantSpeed   float64
	}{
		{"defaults", required, 1, defaultListen, nil, 0},
		{"explicit tip", append(required, "--tip", "3"), 3, defaultListen, nil, 0},
		{"custom listen", append(required, "--listen", ":8080"), 1, ":8080", nil, 0},
		{
			"capture only",
			append(required, "--network", "sepolia", "--listen", "off"),
			1, "", &networks.Sepolia, 0,
		},
		{
			"network case insensitive",
			append(required, "--network", "MAINNET"),
			1, defaultListen, &networks.Mainnet, 0,
		},
		{"speed replay", append(required, "--speed", "2.5"), 1, defaultListen, nil, 2.5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := parseConfig(t, test.args...)
			require.NoError(t, err)
			require.Equal(t, "d", parsed.data)
			require.Equal(t, uint64(1), parsed.from)
			require.Equal(t, uint64(3), parsed.to)
			require.Equal(t, test.wantTip, parsed.tip)
			require.Equal(t, test.wantListen, parsed.listen)
			require.Equal(t, test.wantNetwork, parsed.network)
			require.Equal(t, test.wantSpeed, parsed.speed)
			require.Equal(t, defaultInterval, parsed.interval)
		})
	}
}

func TestValidateCaptureDefaults(t *testing.T) {
	parsed, err := parseConfig(t, "--data", "d", "--from", "1", "--to", "3")
	require.NoError(t, err)
	require.Equal(t, defaultConcurrency, parsed.concurrency)
	require.Equal(t, defaultCaptureTimeout, parsed.captureTimeout)
	require.Equal(t, defaultCaptureRetries, parsed.captureRetries)
	require.Equal(t, defaultCaptureRetryDelay, parsed.captureRetryDelay)
	require.Equal(t, time.Duration(0), parsed.latency)
	require.Equal(t, "info", parsed.logLevel.String())
}

func TestValidatePreConfirmed(t *testing.T) {
	required := []string{"--data", "d", "--from", "1", "--to", "3"}
	tests := []struct {
		name             string
		args             []string
		wantPreconfirmed bool
		wantRPC          string
	}{
		{"defaults", required, false, ""},
		{"serve rounds offline", append(required, "--preconfirmed"), true, ""},
		{
			"capture rounds",
			append(required, "--network", "sepolia", "--listen", "off", "--preconfirmed", "--rpc-url", "http://node:6060"),
			true, "http://node:6060",
		},
		{
			"rpc url alone",
			append(required, "--rpc-url", "https://node.example/rpc/v0_10"),
			false, "https://node.example/rpc/v0_10",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := parseConfig(t, test.args...)
			require.NoError(t, err)
			require.Equal(t, test.wantPreconfirmed, parsed.preconfirmed)
			if test.wantRPC == "" {
				require.Nil(t, parsed.rpc)
				return
			}
			require.Equal(t, test.wantRPC, parsed.rpc.String())
		})
	}
}

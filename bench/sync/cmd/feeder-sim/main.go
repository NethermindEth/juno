package main

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := newRootCommand().ExecuteContext(ctx)
	stop()
	if err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	return newCommand(run)
}

func newCommand(run func(ctx context.Context, config *config) error) *cobra.Command {
	config := &config{}
	command := &cobra.Command{
		Use:          "feeder-sim",
		Short:        "Capture feeder gateway responses and replay them behind a simulated chain tip.",
		SilenceUsage: true,
		PreRunE: func(command *cobra.Command, _ []string) error {
			return config.validate(command.Flags())
		},
		RunE: func(command *cobra.Command, _ []string) error {
			return run(command.Context(), config)
		},
	}
	config.register(command)
	return command
}

func run(ctx context.Context, config *config) error {
	logger, err := log.NewZapLogger(config.logLevel, log.WithColour(true))
	if err != nil {
		return err
	}
	dataset := dataset{root: config.data}

	if config.network != nil {
		if err := capture(ctx, dataset, config, logger); err != nil {
			return fmt.Errorf("capture: %w", err)
		}
	}
	if config.listen == "" {
		return nil
	}
	return serve(ctx, dataset, config, logger)
}

func serve(ctx context.Context, dataset dataset, config *config, logger *log.ZapLogger) error {
	store, err := loadStore(ctx, dataset, config, logger)
	if err != nil {
		return err
	}

	clock := newClock(store.blocks, config, logger)

	var window *window
	if config.preconfirmed {
		if window, err = newWindow(store, config, logger, clock.position()); err != nil {
			return err
		}
	}

	server := &server{store: store, clock: clock, window: window, config: config, logger: logger}

	group, ctx := errgroup.WithContext(ctx)
	if window != nil {
		group.Go(func() error { window.run(ctx, clock.advanced); return nil })
	}
	group.Go(func() error { clock.run(ctx); return nil })
	group.Go(func() error { return server.run(ctx, config.listen) })
	return group.Wait()
}

func junoFlags(network *networks.Network, listen string, preconfirmed bool) string {
	base := "http://" + cmp.Or(listen, defaultListen)
	flags := []string{
		"--cn-name", network.Name,
		"--cn-feeder-url", base + feederPrefix,
		"--cn-gateway-url", base + "/gateway/",
		"--cn-l2-chain-id", network.L2ChainID,
		"--cn-l1-chain-id", network.L1ChainID.String(),
		"--cn-core-contract-address", hexAddress(network.CoreContractAddress),
		"--cn-unverifiable-range", "0,0",
	}

	if !preconfirmed {
		flags = append(flags, "--preconfirmed-poll-interval", "0")
	}

	return strings.Join(flags, " ")
}

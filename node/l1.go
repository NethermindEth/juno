package node

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils/log"
)

// l1StateProviderFull is the surface one provider instance serves: the sync
// loop (l1.L1StateProvider) and the RPC handlers (rpccore.L1Client).
type l1StateProviderFull interface {
	l1.L1StateProvider
	rpccore.L1Client
}

func newL1Client(
	ctx context.Context,
	useNewL1Client bool,
	ethNode string,
	includeMetrics bool,
	chain *blockchain.Blockchain,
	logger log.StructuredLogger,
) (*l1.Client, l1StateProviderFull, error) {
	// One EventListener shared by the L1 client (OnNewL1Head) and the
	// provider (OnL1Call), wired only under --metrics.
	l1Opts := []l1.Option{}
	var listener l1.EventListener
	if includeMetrics {
		listener = makeL1Metrics(chain)
		l1Opts = append(l1Opts, l1.WithEventListener(listener))
	}

	var provider l1StateProviderFull
	var err error
	if useNewL1Client {
		var providerOpts []l1.EthL1StateProviderOption
		if includeMetrics {
			providerOpts = append(providerOpts, l1.WithEthL1StateProviderListener(listener))
		}
		provider, err = newEthL1StateProvider(ctx, ethNode, chain, providerOpts...)
	} else {
		var providerOpts []l1.GethL1StateProviderOption
		if includeMetrics {
			providerOpts = append(providerOpts, l1.WithL1StateProviderListener(listener))
		}
		provider, err = newGethL1StateProvider(ctx, ethNode, chain, providerOpts...)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("creating L1 state provider: %w", err)
	}
	if includeMetrics {
		registerL1Metrics(provider)
	}

	return l1.NewClient(provider, chain, logger, l1Opts...), provider, nil
}

func newMigrationL1StateProvider(
	ctx context.Context,
	useNewL1Client bool,
	ethNode string,
	chain *blockchain.Blockchain,
) (l1.L1StateProvider, error) {
	if useNewL1Client {
		return newEthL1StateProvider(ctx, ethNode, chain)
	}
	return newGethL1StateProvider(ctx, ethNode, chain)
}

func newGethL1StateProvider(
	ctx context.Context,
	ethNode string,
	chain *blockchain.Blockchain,
	opts ...l1.GethL1StateProviderOption,
) (*l1.GethL1StateProvider, error) {
	if err := validateWSURL(ethNode); err != nil {
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	provider, err := l1.NewGethL1StateProvider(
		dialCtx, ethNode, chain.Network().CoreContractAddress, opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("dialing L1 state provider: %w", err)
	}
	return provider, nil
}

func newEthL1StateProvider(
	ctx context.Context,
	ethNode string,
	chain *blockchain.Blockchain,
	opts ...l1.EthL1StateProviderOption,
) (*l1.EthL1StateProvider, error) {
	if err := validateWSURL(ethNode); err != nil {
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	provider, err := l1.NewEthL1StateProvider(
		dialCtx, ethNode, chain.Network().CoreContractAddress, opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("dialing L1 state provider: %w", err)
	}
	return provider, nil
}

// validateWSURL rejects non-websocket URLs: eth_subscribe needs a long-lived
// connection HTTP can't provide.
func validateWSURL(ethNode string) error {
	ethNodeURL, err := url.Parse(ethNode)
	if err != nil {
		return fmt.Errorf("parsing Ethereum node URL: %w", err)
	}
	if ethNodeURL.Scheme != "wss" && ethNodeURL.Scheme != "ws" {
		return errors.New("non-websocket Ethereum node URL (need wss://... or ws://...)")
	}
	return nil
}

package node

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/utils/log"
)

func newL1Client(
	ctx context.Context,
	ethNode string,
	includeMetrics bool,
	chain *blockchain.Blockchain,
	logger log.StructuredLogger,
) (*l1.Client, *l1.GethL1StateProvider, error) {
	// One EventListener, shared by the L1 client (OnNewL1Head) and
	// the provider (OnL1Call), wired only under --metrics.
	l1Opts := []l1.Option{}
	providerOpts := []l1.GethL1StateProviderOption{}
	if includeMetrics {
		listener := makeL1Metrics(chain)
		l1Opts = append(l1Opts, l1.WithEventListener(listener))
		providerOpts = append(providerOpts, l1.WithL1StateProviderListener(listener))
	}

	provider, err := newGethL1StateProvider(ctx, ethNode, chain, providerOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("creating L1 state provider: %w", err)
	}
	if includeMetrics {
		registerL1Metrics(provider)
	}

	return l1.NewClient(provider, chain, logger, l1Opts...), provider, nil
}

// newGethL1StateProvider validates the Ethereum endpoint URL and dials the L1
// client. ws/wss is enforced at the URL level because subscribe-based
// log delivery (eth_subscribe) requires a long-lived connection that
// HTTP doesn't provide.
func newGethL1StateProvider(
	ctx context.Context,
	ethNode string,
	chain *blockchain.Blockchain,
	opts ...l1.GethL1StateProviderOption,
) (*l1.GethL1StateProvider, error) {
	ethNodeURL, err := url.Parse(ethNode)
	if err != nil {
		return nil, fmt.Errorf("parsing Ethereum node URL: %w", err)
	}
	if ethNodeURL.Scheme != "wss" && ethNodeURL.Scheme != "ws" {
		return nil, errors.New(
			"non-websocket Ethereum node URL (need wss://... or ws://...)",
		)
	}

	// One-minute timeout layered on the caller's ctx so a slow dial
	// can't outlive node startup or the migration that triggered it.
	dialCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	provider, err := l1.NewGethL1StateProvider(
		dialCtx, ethNode, chain.Network().CoreContractAddress, opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("setting up L1 state provider: %w", err)
	}
	return provider, nil
}

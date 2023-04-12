package utils

import (
	"encoding"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/spf13/pflag"
)

var ErrUnknownNetwork = errors.New("unknown network (known: mainnet, goerli, goerli2, integration)")

type Network int

// The following are necessary for Cobra and Viper, respectively, to unmarshal log level
// CLI/config parameters properly.
var (
	_ pflag.Value              = (*Network)(nil)
	_ encoding.TextUnmarshaler = (*Network)(nil)
)

const (
	MAINNET Network = iota + 1
	GOERLI
	GOERLI2
	INTEGRATION
)

func (n Network) String() string {
	switch n {
	case MAINNET:
		return "mainnet"
	case GOERLI:
		return "goerli"
	case GOERLI2:
		return "goerli2"
	case INTEGRATION:
		return "integration"
	default:
		// Should not happen.
		panic(ErrUnknownNetwork)
	}
}

func (n *Network) Set(s string) error {
	switch s {
	case "MAINNET", "mainnet":
		*n = MAINNET
	case "GOERLI", "goerli":
		*n = GOERLI
	case "GOERLI2", "goerli2":
		*n = GOERLI2
	case "INTEGRATION", "integration":
		*n = INTEGRATION
	default:
		return ErrUnknownNetwork
	}
	return nil
}

func (n *Network) Type() string {
	return "Network"
}

func (n *Network) UnmarshalText(text []byte) error {
	return n.Set(string(text))
}

// baseURL returns the base URL without endpoint
func (n Network) baseURL() string {
	switch n {
	case GOERLI:
		return "https://alpha4.starknet.io/"
	case MAINNET:
		return "https://alpha-mainnet.starknet.io/"
	case GOERLI2:
		return "https://alpha4-2.starknet.io/"
	case INTEGRATION:
		return "https://external.integration.starknet.io/"
	default:
		// Should not happen.
		panic(ErrUnknownNetwork)
	}
}

// FeederURL returns URL for read commands
func (n Network) FeederURL() string {
	return n.baseURL() + "feeder_gateway/"
}

// GatewayURL returns URL for write commands
func (n Network) GatewayURL() string {
	return n.baseURL() + "gateway/"
}

func (n Network) ChainIDString() string {
	switch n {
	case GOERLI, INTEGRATION:
		return "SN_GOERLI"
	case MAINNET:
		return "SN_MAIN"
	case GOERLI2:
		return "SN_GOERLI2"
	default:
		// Should not happen.
		panic(ErrUnknownNetwork)
	}
}

func (n Network) ChainID() *felt.Felt {
	return new(felt.Felt).SetBytes([]byte(n.ChainIDString()))
}

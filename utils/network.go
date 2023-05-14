package utils

import (
	"encoding"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common"
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

func (n Network) ChainID() *felt.Felt {
	switch n {
	case GOERLI, INTEGRATION:
		return new(felt.Felt).SetBytes([]byte("SN_GOERLI"))
	case MAINNET:
		return new(felt.Felt).SetBytes([]byte("SN_MAIN"))
	case GOERLI2:
		return new(felt.Felt).SetBytes([]byte("SN_GOERLI2"))
	default:
		// Should not happen.
		panic(ErrUnknownNetwork)
	}
}

func (n Network) CoreContractAddress() (common.Address, error) {
	var address common.Address
	// The docs states the addresses for each network: https://docs.starknet.io/documentation/useful_info/
	switch n {
	case MAINNET:
		address = common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4")
	case GOERLI:
		address = common.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e")
	case GOERLI2:
		address = common.HexToAddress("0xa4eD3aD27c294565cB0DCc993BDdCC75432D498c")
	case INTEGRATION:
		return common.Address{}, errors.New("l1 contract is not available on the integration network")
	default:
		// Should not happen.
		return common.Address{}, ErrUnknownNetwork
	}
	return address, nil
}

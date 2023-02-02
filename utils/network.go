package utils

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
)

var ErrUnknownNetwork = errors.New("unknown network")

type Network uint8

const (
	MAINNET Network = iota
	GOERLI
	GOERLI2
	INTEGRATION
)

func (n Network) String() string {
	switch n {
	case GOERLI:
		return "goerli"
	case MAINNET:
		return "mainnet"
	case GOERLI2:
		return "goerli2"
	case INTEGRATION:
		return "integration"
	default:
		return ""
	}
}

func (n Network) URL() string {
	switch n {
	case GOERLI:
		return "https://alpha4.starknet.io/feeder_gateway/"
	case MAINNET:
		return "https://alpha-mainnet.starknet.io/feeder_gateway/"
	case GOERLI2:
		return "https://alpha4-2.starknet.io/feeder_gateway/"
	case INTEGRATION:
		return "https://external.integration.starknet.io/feeder_gateway/"
	default:
		return ""
	}
}

func (n Network) ChainId() *felt.Felt {
	switch n {
	case GOERLI:
		return new(felt.Felt).SetBytes([]byte("SN_GOERLI"))
	case MAINNET:
		return new(felt.Felt).SetBytes([]byte("SN_MAIN"))
	case GOERLI2:
		return new(felt.Felt).SetBytes([]byte("SN_GOERLI2"))
	case INTEGRATION:
		return new(felt.Felt).SetBytes([]byte("SN_INTEGRATION"))
	default:
		return nil
	}
}

func IsValidNetwork(n Network) bool {
	return !(n.String() == "")
}

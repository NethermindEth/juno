package utils

type Network uint8

const (
	GOERLI Network = iota
	MAINNET
)

func (n Network) String() string {
	switch n {
	case GOERLI:
		return "goerli"
	case MAINNET:
		return "mainnet"
	default:
		return ""
	}
}

func (n Network) URL() string {
	switch n {
	case GOERLI:
		return "https://alpha4.starknet.io"
	case MAINNET:
		return "https://alpha-mainnet.starknet.io"
	default:
		return ""
	}
}

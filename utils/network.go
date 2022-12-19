package utils

type Network uint8

const (
	GOERLI Network = iota
	MAINNET
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
		return "https://alpha4.starknet.io"
	case MAINNET:
		return "https://alpha-mainnet.starknet.io"
	case GOERLI2:
		return "https://alpha4.starknet.io"
	case INTEGRATION:
		return "https://external.integration.starknet.io"
	default:
		return ""
	}
}

type Chain string

const (
	Goerli      Chain = "SN_GOERLI"
	Mainnet     Chain = "SN_MAINNET"
	Goerli2     Chain = "SN_GOERLI2"
	Integration Chain = "SN_INTEGRATION"
)

func (n Network) ChainId() Chain {
	switch n {
	case GOERLI:
		return Goerli
	case MAINNET:
		return Mainnet
	case GOERLI2:
		return Goerli2
	case INTEGRATION:
		return Integration
	default:
		return ""
	}
}

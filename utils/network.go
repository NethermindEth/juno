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

func (n Network) ChainId() string {
	switch n {
	case GOERLI:
		return "SN_GOERLI"
	case MAINNET:
		return "SN_MAINNET"
	case GOERLI2:
		return "SN_GOERLI2"
	case INTEGRATION:
		return "SN_INTEGRATION"
	default:
		return ""
	}
}

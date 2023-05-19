package utils_test

import (
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var networkStrings = map[utils.Network]string{
	utils.MAINNET:     "mainnet",
	utils.GOERLI:      "goerli",
	utils.GOERLI2:     "goerli2",
	utils.INTEGRATION: "integration",
}

func TestNetwork(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		for network, str := range networkStrings {
			assert.Equal(t, str, network.String())
		}
	})
	t.Run("url", func(t *testing.T) {
		for n := range networkStrings {
			switch n {
			case utils.GOERLI:
				assert.Equal(t, "https://alpha4.starknet.io/feeder_gateway/", n.URL())
			case utils.MAINNET:
				assert.Equal(t, "https://alpha-mainnet.starknet.io/feeder_gateway/", n.URL())
			case utils.GOERLI2:
				assert.Equal(t, "https://alpha4-2.starknet.io/feeder_gateway/", n.URL())
			case utils.INTEGRATION:
				assert.Equal(t, "https://external.integration.starknet.io/feeder_gateway/", n.URL())
			default:
				assert.Fail(t, "unexpected network")
			}
		}
	})
	t.Run("chainId", func(t *testing.T) {
		for n := range networkStrings {
			switch n {
			case utils.GOERLI, utils.INTEGRATION:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_GOERLI")), n.ChainID())
			case utils.MAINNET:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_MAIN")), n.ChainID())
			case utils.GOERLI2:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_GOERLI2")), n.ChainID())
			default:
				assert.Fail(t, "unexpected network")
			}
		}
	})
}

//nolint:dupl // see comment in utils/log_test.go
func TestNetworkSet(t *testing.T) {
	for network, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) {
			n := new(utils.Network)
			require.NoError(t, n.Set(str))
			assert.Equal(t, network, *n)
		})
		uppercase := strings.ToUpper(str)
		t.Run("network "+uppercase, func(t *testing.T) {
			n := new(utils.Network)
			require.NoError(t, n.Set(uppercase))
			assert.Equal(t, network, *n)
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		n := new(utils.Network)
		require.ErrorIs(t, n.Set("blah"), utils.ErrUnknownNetwork)
	})
}

func TestNetworkUnmarshalText(t *testing.T) {
	for network, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) {
			n := new(utils.Network)
			require.NoError(t, n.UnmarshalText([]byte(str)))
			assert.Equal(t, network, *n)
		})
		uppercase := strings.ToUpper(str)
		t.Run("network "+uppercase, func(t *testing.T) {
			n := new(utils.Network)
			require.NoError(t, n.UnmarshalText([]byte(uppercase)))
			assert.Equal(t, network, *n)
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		l := new(utils.Network)
		require.ErrorIs(t, l.UnmarshalText([]byte("blah")), utils.ErrUnknownNetwork)
	})
}

func TestNetworkType(t *testing.T) {
	assert.Equal(t, "Network", new(utils.Network).Type())
}

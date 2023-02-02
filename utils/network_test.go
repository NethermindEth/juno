package utils_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestNetwork(t *testing.T) {
	networks := []utils.Network{0, 1, 2, 56, 3, 100}
	t.Run("string", func(t *testing.T) {
		for _, n := range networks {
			switch n {
			case utils.GOERLI:
				assert.Equal(t, "goerli", n.String())
			case utils.MAINNET:
				assert.Equal(t, "mainnet", n.String())
			case utils.GOERLI2:
				assert.Equal(t, "goerli2", n.String())
			case utils.INTEGRATION:
				assert.Equal(t, "integration", n.String())
			default:
				assert.Equal(t, "", n.String())
			}
		}
	})
	t.Run("url", func(t *testing.T) {
		for _, n := range networks {
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
				assert.Equal(t, "", n.URL())

			}
		}
	})
	t.Run("chainId", func(t *testing.T) {
		for _, n := range networks {
			switch n {
			case utils.GOERLI:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_GOERLI")), n.ChainId())
			case utils.MAINNET:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_MAIN")), n.ChainId())
			case utils.GOERLI2:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_GOERLI2")), n.ChainId())
			case utils.INTEGRATION:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_INTEGRATION")), n.ChainId())
			default:
				assert.Equal(t, (*felt.Felt)(nil), n.ChainId())
			}
		}
	})
}

func TestValidNetwork(t *testing.T) {
	t.Run("valid networks", func(t *testing.T) {
		networks := []utils.Network{0, 1, 2, 3}
		for _, n := range networks {
			assert.True(t, utils.IsValidNetwork(n))
		}
	})
	t.Run("invalid networks", func(t *testing.T) {
		networks := []utils.Network{6, 5, 7, 12, 34, 255}
		for _, n := range networks {
			assert.False(t, utils.IsValidNetwork(n))
		}
	})
}

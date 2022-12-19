package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetwork(t *testing.T) {
	networks := []Network{0, 1, 2, 56, 3, 100}
	t.Run("string", func(t *testing.T) {
		for _, n := range networks {
			switch n {
			case GOERLI:
				assert.Equal(t, "goerli", n.String())
			case MAINNET:
				assert.Equal(t, "mainnet", n.String())
			case GOERLI2:
				assert.Equal(t, "goerli2", n.String())
			case INTEGRATION:
				assert.Equal(t, "integration", n.String())
			default:
				assert.Equal(t, "", n.String())
			}
		}
	})
	t.Run("url", func(t *testing.T) {
		for _, n := range networks {
			switch n {
			case GOERLI:
				assert.Equal(t, "https://alpha4.starknet.io", n.URL())
			case MAINNET:
				assert.Equal(t, "https://alpha-mainnet.starknet.io", n.URL())
			case GOERLI2:
				assert.Equal(t, "https://alpha4.starknet.io", n.URL())
			case INTEGRATION:
				assert.Equal(t, "https://external.integration.starknet.io", n.URL())
			default:
				assert.Equal(t, "", n.URL())

			}
		}
	})
	t.Run("chainId", func(t *testing.T) {
		for _, n := range networks {
			switch n {
			case GOERLI:
				assert.Equal(t, Goerli, n.ChainId())
			case MAINNET:
				assert.Equal(t, Mainnet, n.ChainId())
			case GOERLI2:
				assert.Equal(t, Goerli2, n.ChainId())
			case INTEGRATION:
				assert.Equal(t, Integration, n.ChainId())
			default:
				assert.Equal(t, Chain(""), n.ChainId())

			}
		}
	})
}

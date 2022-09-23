package utils

import (
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestNetwork(t *testing.T) {
	networks := []Network{0, 1, 2, 56, 3, 100}
	t.Run("string", func(t *testing.T) {
		for _, n := range networks {
			switch n {
			case GOERLI:
				assert.Check(t, is.Equal("goerli", n.String()))
			case MAINNET:
				assert.Check(t, is.Equal("mainnet", n.String()))
			default:
				assert.Check(t, is.Equal("", n.String()))

			}
		}
	})
	t.Run("url", func(t *testing.T) {
		for _, n := range networks {
			switch n {
			case GOERLI:
				assert.Check(t, is.Equal("https://alpha4.starknet.io", n.URL()))
			case MAINNET:
				assert.Check(t, is.Equal("https://alpha-mainnet.starknet.io", n.URL()))
			default:
				assert.Check(t, is.Equal("", n.URL()))

			}
		}
	})
}

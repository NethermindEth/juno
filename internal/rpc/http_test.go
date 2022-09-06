package rpc

import (
	"errors"
	"net"
	"testing"

	"gotest.tools/assert"
)

func TestGetIpAddress(t *testing.T) {
	t.Run("should return error when cannot get address from net.interface", func(t *testing.T) {
		result, err := getIpAddress(NetworkHandler{
			GetInterfaces:           mockGetInterfaces,
			GetAddressFromInterface: mockGetEmptyAddress,
		})

		assert.Assert(t, result == nil, "result should be nil")
		assert.Equal(t, "cannot get addrs", err.Error())
	})
	t.Run("should return the IP provided while mocking", func(t *testing.T) {
		result, err := getIpAddress(NetworkHandler{
			GetInterfaces:           mockGetInterfaces,
			GetAddressFromInterface: mockGetAddress,
		})
		assert.Assert(t, err == nil, "error should be nil")
		assert.Equal(t, result.String(), "192.168.1.1", "IP should be the same mocked")
	})
	t.Run("should return error when cannot remove the mask", func(t *testing.T) {
		result, err := getIpAddress(NetworkHandler{
			GetInterfaces:           mockGetInterfaces,
			GetAddressFromInterface: mockGetAddressWithoutMask,
		})
		assert.Assert(t, result == nil, "result should be nil")
		assert.Equal(t, "network: ip not found", err.Error())
	})
}

func mockGetInterfaces() ([]net.Interface, error) {
	return []net.Interface{
		{Name: "wlan0"},
		{Name: "eth0"},
	}, nil
}

// stub behavior when calling net.Interface{}.Addrs()
func mockGetEmptyAddress(i net.Interface) ([]net.Addr, error) {
	return nil, errors.New("cannot get addrs")
}

func mockGetAddress(i net.Interface) ([]net.Addr, error) {
	addresses := make([]net.Addr, 0)
	addresses = append(addresses, &net.IPNet{IP: net.IPv4(192, 168, 1, 1), Mask: net.IPv4Mask(255, 255, 255, 0)})
	return addresses, nil
}

func mockGetAddressWithoutMask(i net.Interface) ([]net.Addr, error) {
	addresses := make([]net.Addr, 0)
	addresses = append(addresses, &net.IPNet{IP: net.IPv4(192, 168, 1, 1)})
	return addresses, nil
}

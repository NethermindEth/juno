package p2p

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type multiaddrEndpoints struct {
	local  multiaddr.Multiaddr
	remote multiaddr.Multiaddr
}

func (c *multiaddrEndpoints) LocalMultiaddr() multiaddr.Multiaddr {
	return c.local
}

func (c *multiaddrEndpoints) RemoteMultiaddr() multiaddr.Multiaddr {
	return c.remote
}

func TestConnectionGater_AllowAll(t *testing.T) {
	ipAddr := net.ParseIP("127.0.0.1")

	listen, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", ipAddr.String(), 12345))
	require.NoError(t, err)

	connGater, err := newConnectionGater([]string{}, []string{}, utils.NewNopZapLogger())
	require.NoError(t, err)

	h1, err := libp2p.New([]libp2p.Option{libp2p.ListenAddrs(listen), libp2p.ConnectionGater(connGater)}...)
	require.NoError(t, err)
	defer func() {
		err := h1.Close()
		require.NoError(t, err)
	}()

	listen, err = multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", ipAddr.String(), 12345))
	require.NoError(t, err)

	h2, err := libp2p.New([]libp2p.Option{libp2p.ListenAddrs(listen)}...)
	require.NoError(t, err)
	defer func() {
		err := h2.Close()
		require.NoError(t, err)
	}()

	multiAddress, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", ipAddr, 12345, h2.ID()))
	require.NoError(t, err)
	addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddress)
	require.NoError(t, err)
	err = h1.Connect(context.Background(), *addrInfo)
	assert.NotNil(t, err, "Connection shouldn't be forbidden by connection gater")
}

func TestConnectionGater_DenyPeer(t *testing.T) {
	var (
		ipAddr = net.ParseIP("127.0.0.1")
		cidr   = "127.0.0.0/8"
	)

	listen, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", ipAddr.String(), 12345))
	require.NoError(t, err)

	connGater, err := newConnectionGater([]string{cidr}, []string{}, utils.NewNopZapLogger())
	require.NoError(t, err)

	h1, err := libp2p.New([]libp2p.Option{libp2p.ListenAddrs(listen), libp2p.ConnectionGater(connGater)}...)
	require.NoError(t, err)
	defer func() {
		err := h1.Close()
		require.NoError(t, err)
	}()

	listen, err = multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", ipAddr.String(), 12345))
	require.NoError(t, err)

	h2, err := libp2p.New([]libp2p.Option{libp2p.ListenAddrs(listen)}...)
	require.NoError(t, err)
	defer func() {
		err := h2.Close()
		require.NoError(t, err)
	}()

	multiAddress, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", ipAddr, 12345, h2.ID()))
	require.NoError(t, err)
	addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddress)
	require.NoError(t, err)
	err = h1.Connect(context.Background(), *addrInfo)
	assert.NotNil(t, err, "Connection should be forbidden by connection gater")
}

func TestConnectionGater(t *testing.T) {
	tests := []struct {
		name         string
		denyCIDR     []string
		allowCIDR    []string
		addr         string
		expectDial   bool
		expectAccept bool
	}{
		{
			name:         "Default",
			denyCIDR:     []string{},
			addr:         "/ip4/127.0.0.1/tcp/1234",
			expectDial:   true,
			expectAccept: true,
		},
		{
			name:         "DenyCIDR_SameCidr",
			denyCIDR:     []string{"127.0.0.0/8"},
			addr:         "/ip4/127.0.0.1/tcp/1234",
			expectDial:   false,
			expectAccept: false,
		},
		{
			name:         "DenyCIDR_DifferentCidr",
			denyCIDR:     []string{"192.168.0.0/16"},
			addr:         "/ip4/127.0.0.1/tcp/1234",
			expectDial:   true,
			expectAccept: true,
		},
		{
			name:         "AllowCIDR_SameCIDR",
			allowCIDR:    []string{"127.0.0.0/8"},
			addr:         "/ip4/127.0.0.1/tcp/1234",
			expectDial:   true,
			expectAccept: true,
		},
		{
			name:         "AllowCIDR_DifferentCidr",
			allowCIDR:    []string{"192.168.0.0/16"},
			addr:         "/ip4/127.0.0.1/tcp/1234",
			expectDial:   false,
			expectAccept: false,
		},
		{
			name:         "Collision",
			allowCIDR:    []string{"127.0.0.0/8"},
			denyCIDR:     []string{"127.0.0.0/8"},
			addr:         "/ip4/127.0.0.1/tcp/1234",
			expectDial:   false,
			expectAccept: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connGater, err := newConnectionGater(tt.denyCIDR, tt.allowCIDR, utils.NewNopZapLogger())
			require.NoError(t, err)

			// Test InterceptAddrDial
			addr, err := multiaddr.NewMultiaddr(tt.addr)
			require.NoError(t, err)
			resultDial := connGater.InterceptAddrDial("", addr)
			require.Equal(t, tt.expectDial, resultDial)

			// Test InterceptAccept
			resultAccept := connGater.InterceptAccept(&multiaddrEndpoints{remote: addr})
			require.Equal(t, tt.expectAccept, resultAccept)
		})
	}
}

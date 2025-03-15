package p2p

import (
	"fmt"
	"net"

	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	multinet "github.com/multiformats/go-multiaddr/net"
)

// connectionGater manages connection filtering based on multiaddr rules.
type connectionGater struct {
	addrFilter *multiaddr.Filters
	log        utils.SimpleLogger
}

func newConnectionGater(denyListCIDR, allowListCIDR []string, log utils.SimpleLogger) (*connectionGater, error) {
	addrFilter, err := configureFilter(denyListCIDR, allowListCIDR, log)
	if err != nil {
		return nil, fmt.Errorf("configure address filter: %w", err)
	}

	return &connectionGater{
		addrFilter: addrFilter,
		log:        log,
	}, nil
}

// InterceptPeerDial always allows dialling a peer.
func (*connectionGater) InterceptPeerDial(_ peer.ID) (allow bool) {
	return true
}

// InterceptAddrDial checks if dialling a specific multiaddr for a given peer is allowed.
func (s *connectionGater) InterceptAddrDial(pid peer.ID, m multiaddr.Multiaddr) (allow bool) {
	return filterMultiAddr(s.addrFilter, m, s.log)
}

// InterceptAccept checks if an inbound connection is allowed based on its remote address.
func (s *connectionGater) InterceptAccept(n network.ConnMultiaddrs) (allow bool) {
	return filterMultiAddr(s.addrFilter, n.RemoteMultiaddr(), s.log)
}

// InterceptSecured allows all authenticated connections.
func (*connectionGater) InterceptSecured(_ network.Direction, _ peer.ID, _ network.ConnMultiaddrs) (allow bool) {
	return true
}

// InterceptUpgraded allows all fully capable connections.
func (*connectionGater) InterceptUpgraded(_ network.Conn) (allow bool, reason control.DisconnectReason) {
	// A zero value stands for "no reason" / NA.
	return true, 0
}

// configureFilter sets up a filter using deny and allow lists of CIDRs.
// The deny list takes priority: CIDRs in both lists will be blocked.
func configureFilter(denyListCIDR, allowListCIDR []string, log utils.SimpleLogger) (*multiaddr.Filters, error) {
	filter := multiaddr.NewFilters()

	for _, cidr := range allowListCIDR {
		if err := addCIDRToFilter(filter, cidr, multiaddr.ActionAccept, log); err != nil {
			return nil, err
		}
	}

	for _, cidr := range denyListCIDR {
		if err := addCIDRToFilter(filter, cidr, multiaddr.ActionDeny, log); err != nil {
			return nil, err
		}
	}

	return filter, nil
}

// addCIDRToFilter adds a single CIDR to the filter with the specified action.
func addCIDRToFilter(filter *multiaddr.Filters, cidr string, action multiaddr.Action, log utils.SimpleLogger) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	existingAction, _ := filter.ActionForFilter(*ipNet)
	if existingAction != action && existingAction != multiaddr.ActionNone {
		log.Warnw("Conflicting filter rule", "address", ipNet.String(), "current_action", existingAction, "new_action", action)
	}
	filter.AddFilter(*ipNet, action)
	return nil
}

// filterMultiAddr checks the IP subnets in the filter.
// By default libp2p accepts all incoming dials.
func filterMultiAddr(filter *multiaddr.Filters, addr multiaddr.Multiaddr, log utils.SimpleLogger) bool {
	acceptIPNets := filter.FiltersForAction(multiaddr.ActionAccept)

	// If an allow list exists, only connections from specified subnets are permitted, all others are rejected.
	if len(acceptIPNets) > 0 {
		ip, err := multinet.ToIP(addr)
		if err != nil {
			log.Tracew("Multiaddress invalid ip", "error", err)
			return false
		}

		for _, ipnet := range acceptIPNets {
			if ipnet.Contains(ip) {
				return true
			}
		}
		return false
	}

	return !filter.AddrBlocked(addr)
}

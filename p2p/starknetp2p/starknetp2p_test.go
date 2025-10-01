package starknetp2p_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/p2p/starknetp2p"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"
)

var networks = []struct {
	network *utils.Network
	name    string
}{
	{&utils.Mainnet, "SN_MAIN"},
	{&utils.Goerli, "SN_GOERLI"},
	{&utils.Goerli2, "SN_GOERLI2"},
	{&utils.Integration, "SN_GOERLI"},
	{&utils.Sepolia, "SN_SEPOLIA"},
	{&utils.SepoliaIntegration, "SN_INTEGRATION_SEPOLIA"},
}

func TestSync(t *testing.T) {
	subProtocols := []struct {
		protocol starknetp2p.SyncSubProtocol
		name     protocol.ID
	}{
		{starknetp2p.HeadersSyncSubProtocol, "headers"},
		{starknetp2p.StateDiffSyncSubProtocol, "state_diffs"},
		{starknetp2p.ClassesSyncSubProtocol, "classes"},
		{starknetp2p.TransactionsSyncSubProtocol, "transactions"},
		{starknetp2p.EventsSyncSubProtocol, "events"},
	}

	for _, network := range networks {
		for _, protocol := range subProtocols {
			testName := fmt.Sprintf("Sub-protocol %s in network %s", protocol.name, network.name)
			t.Run(testName, func(t *testing.T) {
				require.Equal(
					t,
					fmt.Sprintf("/starknet/%s/sync/%s/0.1.0-rc.0", network.name, protocol.name),
					string(starknetp2p.Sync(network.network, protocol.protocol)),
				)
			})
		}
	}
}

func TestDHT(t *testing.T) {
	protocols := []struct {
		protocol starknetp2p.Protocol
		name     string
	}{
		{starknetp2p.ConsensusProtocolID, "consensus"},
		{starknetp2p.MempoolProtocolID, "mempool"},
		{starknetp2p.SyncProtocolID, "sync"},
	}

	for _, network := range networks {
		for _, p := range protocols {
			testName := fmt.Sprintf("Protocol %s in network %s", p.name, network.name)
			t.Run(testName, func(t *testing.T) {
				options := starknetp2p.DHT(network.network, p.protocol)
				options = append(options, dht.Mode(dht.ModeServer))

				host, err := libp2p.New()
				require.NoError(t, err)

				_, err = dht.New(t.Context(), host, options...)
				require.NoError(t, err)

				require.Contains(
					t,
					host.Mux().Protocols(),
					protocol.ID(fmt.Sprintf("/starknet/%s/%s/kad/1.0.0", network.name, p.name)),
				)
			})
		}
	}
}

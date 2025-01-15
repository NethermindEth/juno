package sync

import (
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestProtocolIDs(t *testing.T) {
	testCases := []struct {
		name     string
		network  *utils.Network
		pidFunc  func(*utils.Network) string
		expected string
	}{
		{
			name:     "HeadersPID with SN_MAIN",
			network:  &utils.Mainnet,
			pidFunc:  func(n *utils.Network) string { return string(HeadersPID(n)) },
			expected: "/starknet/SN_MAIN/sync/headers/0.1.0-rc.0",
		},
		{
			name:     "EventsPID with SN_MAIN",
			network:  &utils.Mainnet,
			pidFunc:  func(n *utils.Network) string { return string(EventsPID(n)) },
			expected: "/starknet/SN_MAIN/sync/events/0.1.0-rc.0",
		},
		{
			name:     "TransactionsPID with SN_MAIN",
			network:  &utils.Mainnet,
			pidFunc:  func(n *utils.Network) string { return string(TransactionsPID(n)) },
			expected: "/starknet/SN_MAIN/sync/transactions/0.1.0-rc.0",
		},
		{
			name:     "ClassesPID with SN_MAIN",
			network:  &utils.Mainnet,
			pidFunc:  func(n *utils.Network) string { return string(ClassesPID(n)) },
			expected: "/starknet/SN_MAIN/sync/classes/0.1.0-rc.0",
		},
		{
			name:     "StateDiffPID with SN_MAIN",
			network:  &utils.Mainnet,
			pidFunc:  func(n *utils.Network) string { return string(StateDiffPID(n)) },
			expected: "/starknet/SN_MAIN/sync/state_diffs/0.1.0-rc.0",
		},
		{
			name:     "DHTPrefixPID with SN_MAIN",
			network:  &utils.Mainnet,
			pidFunc:  func(n *utils.Network) string { return string(DHTPrefixPID(n)) },
			expected: "/starknet/SN_MAIN/sync",
		},
		{
			name:     "HeadersPID with SN_SEPOLIA",
			network:  &utils.Sepolia,
			pidFunc:  func(n *utils.Network) string { return string(HeadersPID(n)) },
			expected: "/starknet/SN_SEPOLIA/sync/headers/0.1.0-rc.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.pidFunc(tc.network)
			assert.Equal(t, tc.expected, result)
		})
	}
}

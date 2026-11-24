package rpctest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/adapters/adaptfeeder"
	"github.com/NethermindEth/juno/adapters/adaptfeeder/adaptfeedertest"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

// NewSepolia returns a Node with the first n Sepolia blocks from the feeder fixtures. Its
// synchronizer does not run, and it reads the traces of old blocks from the feeder fixtures.
// The fixtures do not hold every class: the node does not store a class with no fixture file,
// so a call that needs such a class fails.
func NewSepolia(t testing.TB, n uint64) *Node {
	t.Helper()
	network := &networks.Sepolia
	logger := log.NewNopZapLogger()
	chain := blockchain.New(memory.New(), network,
		blockchain.WithNewState(statetestutils.UseNewState()))

	root, err := repoRoot()
	require.NoError(t, err)
	classDir := filepath.Join(root, "clients", "feeder", "testdata", network.String(), "class")

	client := feeder.NewTestClient(t, network)
	gateway := adaptfeeder.New(client)
	for number := range n {
		block := adaptfeedertest.Block(t, client, number)
		stateUpdate := adaptfeedertest.StateUpdate(t, client, number)
		classes := fixtureClasses(t, chain, gateway, classDir, stateUpdate.StateDiff)
		commitments, err := chain.SanityCheckNewHeight(block, stateUpdate, classes)
		require.NoError(t, err)
		require.NoError(t, chain.Store(
			block, commitments, stateUpdate, classes, core.EventsBloom(block.Receipts),
		))
	}

	synchronizer := sync.New(chain, sync.NewFeederGatewayDataSource(chain, gateway), gateway, logger)
	return newNode(t, chain, newHandler(chain, synchronizer, network, logger).WithFeeder(client))
}

func fixtureClasses(
	t testing.TB,
	chain *blockchain.Blockchain,
	gateway *adaptfeeder.Feeder,
	classDir string,
	stateDiff *core.StateDiff,
) map[felt.ClassHash]core.ClassDefinition {
	t.Helper()
	classHashes, err := sync.MissingClassHashes(chain, stateDiff)
	require.NoError(t, err)

	classes := make(map[felt.ClassHash]core.ClassDefinition, len(classHashes))
	for _, classHash := range classHashes {
		fixture := filepath.Join(classDir, classHash.String()+".json")
		if _, err := os.Stat(fixture); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		class, err := gateway.Class(t.Context(), classHash)
		require.NoError(t, err)
		classes[*classHash] = class
	}
	return classes
}

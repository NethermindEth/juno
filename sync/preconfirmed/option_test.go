package preconfirmed_test

import (
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// A poller built without options runs on the package defaults: it ticks every
// DefaultPollInterval, reports the poll to a listener that drops it, and a read within
// DefaultStaleAfter of the tick is served from the stored chain without polling again.
func TestPollerDefaults(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	block1 := makeTestPreConfirmedBlock("r0", 1)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockPreConfirmedDataSource(ctrl)
	ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
		Return(block1, uint64(1), nil)

	synctest.Test(t, func(t *testing.T) {
		highest := &atomic.Pointer[core.Header]{}
		highest.Store(fx.head)
		poller := preconfirmed.NewPoller(ds, fx.bc, highest, log.NewNopZapLogger())

		go poller.Run(t.Context())
		synctest.Wait()
		time.Sleep(preconfirmed.DefaultPollInterval)
		synctest.Wait()

		time.Sleep(preconfirmed.DefaultStaleAfter / 2)
		view, err := poller.PreConfirmedChain()
		require.NoError(t, err)
		// Lets a poll the read may have asked for run before the test ends, so it is seen.
		synctest.Wait()
		assertChain(t, &view, entry(1, &block1))
	})
}

package feeder

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMigrationFeeder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := mocks.NewMockFeederReader(ctrl)
		blockNumber := uint64(1234)
		blockNumberStr := strconv.FormatUint(blockNumber, 10)

		mf := NewMigrationFeeder(New(mockClient))

		// ************************************************/
		// ***** uses legacy two-call path by default *****/
		// ************************************************/
		mockClient.EXPECT().
			StateUpdateWithBlock(gomock.Any(), blockNumberStr).
			Return(emptyStateUpdate(), nil)
		mockClient.EXPECT().
			Signature(gomock.Any(), blockNumberStr).
			Return(emptySignature(), nil)

		su, blk, err := mf.StateUpdateWithBlock(t.Context(), blockNumber)
		require.NoError(t, err)
		require.NotNil(t, su)
		require.NotNil(t, blk)

		// ************************************************/
		// ****** starting verification loop service. *****/
		// ************************************************/

		// The first feeder call for the new endpoint starts immediately.
		// Here, we simulate a failure, meaning the feeder is not updated yet.
		mockClient.EXPECT().
			StateUpdateWithBlockAndSignature(gomock.Any(), "0").
			Return(nil, errors.New("mock error"))

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1) // it will be used later in the test
		go func() { done <- mf.Run(ctx) }()
		synctest.Wait()

		// Since the feeder is not updated, the code will fall back to the legacy two-call path.
		mockClient.EXPECT().
			StateUpdateWithBlock(gomock.Any(), blockNumberStr).
			Return(emptyStateUpdate(), nil)
		mockClient.EXPECT().
			Signature(gomock.Any(), blockNumberStr).
			Return(emptySignature(), nil)

		su, blk, err = mf.StateUpdateWithBlock(t.Context(), blockNumber)
		require.NoError(t, err)
		require.NotNil(t, su)
		require.NotNil(t, blk)

		// ************************************************************************/
		// ****** time has passed, 1 second before next verification interval *****/
		// ************************************************************************/
		timePassed := verificationInterval - time.Second
		time.Sleep(timePassed)
		synctest.Wait()

		// The legacy two-call path is still being used.
		mockClient.EXPECT().
			StateUpdateWithBlock(gomock.Any(), blockNumberStr).
			Return(emptyStateUpdate(), nil)
		mockClient.EXPECT().
			Signature(gomock.Any(), blockNumberStr).
			Return(emptySignature(), nil)

		su, blk, err = mf.StateUpdateWithBlock(t.Context(), blockNumber)
		require.NoError(t, err)
		require.NotNil(t, su)
		require.NotNil(t, blk)

		// **************************************************************/
		// ******* 1st verification tick, feeder is not updated yet *****/
		// **************************************************************/
		mockClient.EXPECT().
			StateUpdateWithBlockAndSignature(gomock.Any(), "0").
			Return(nil, errors.New("mock error"))
		time.Sleep(time.Second)
		synctest.Wait()

		// The legacy two-call path is still being used.
		mockClient.EXPECT().
			StateUpdateWithBlock(gomock.Any(), blockNumberStr).
			Return(emptyStateUpdate(), nil)
		mockClient.EXPECT().
			Signature(gomock.Any(), blockNumberStr).
			Return(emptySignature(), nil)

		su, blk, err = mf.StateUpdateWithBlock(t.Context(), blockNumber)
		require.NoError(t, err)
		require.NotNil(t, su)
		require.NotNil(t, blk)

		// ******************************************************/
		// ******* 2nd verification tick, feeder is updated *****/
		// ******************************************************/
		mockClient.EXPECT().
			StateUpdateWithBlockAndSignature(gomock.Any(), "0").
			Return(emptyStateUpdateWithSig(), nil)

		time.Sleep(verificationInterval)
		synctest.Wait()

		// The new endpoint is being used.
		mockClient.EXPECT().
			StateUpdateWithBlockAndSignature(gomock.Any(), blockNumberStr).
			Return(emptyStateUpdateWithSig(), nil)

		su, blk, err = mf.StateUpdateWithBlock(t.Context(), blockNumber)
		require.NoError(t, err)
		require.NotNil(t, su)
		require.NotNil(t, blk)

		// *******************************************************************/
		// ******* verification loop should stop after feeder is updated *****/
		// *******************************************************************/

		// We wait for 5 verification intervals. If the verification loop is running,
		// it will trigger 5 calls to the new endpoint, and since we haven't
		// configured any expected calls for the feeder mock, the test will fail.
		time.Sleep(verificationInterval * 5)
		synctest.Wait()

		// ***************************************************/
		// ******* Run must block until ctx is cancelled *****/
		// ***************************************************/

		// Even though the verification loop is stopped, the Run method must
		// block until ctx is cancelled.
		select {
		case <-done:
			t.Fatal("Run returned before ctx cancellation")
		default:
		}

		cancel()
		synctest.Wait()

		select {
		case <-done:
		default:
			t.Fatal("Run did not return after ctx cancellation")
		}
	})
}

func emptyStateUpdateWithSig() *starknet.StateUpdateWithBlockAndSignature {
	stateUpdate := emptyStateUpdate()
	return &starknet.StateUpdateWithBlockAndSignature{
		StateUpdate: stateUpdate.StateUpdate,
		Block:       stateUpdate.Block,
		Signature:   emptySignature().Signature,
	}
}

func emptyStateUpdate() *starknet.StateUpdateWithBlock {
	return &starknet.StateUpdateWithBlock{
		Block: &starknet.Block{
			Hash: &felt.Zero,
		},
		StateUpdate: &starknet.StateUpdate{
			StateDiff: starknet.StateDiff{},
		},
	}
}

func emptySignature() *starknet.Signature {
	return &starknet.Signature{Signature: []*felt.Felt{&felt.Zero, &felt.Zero}}
}

package feeder_test

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
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	latestID             = "latest"
	verificationInterval = 30 * time.Minute // same as production code
)

func TestFeederAdapter_FullCycle_StateUpdateWithBlock(t *testing.T) {
	blockNumber := uint64(1234)
	blockNumberStr := strconv.FormatUint(blockNumber, 10)

	t.Run(
		"full cycle - start with an outdated feeder and it gets updated afterwards",
		func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				mockClient := mocks.NewMockFeederReader(ctrl)

				fa := adaptfeeder.NewFeederAdapter(adaptfeeder.New(mockClient), log.NewNopZapLogger())

				// ************************************************/
				// ***** uses legacy two-call path by default *****/
				// ************************************************/
				mockClient.EXPECT().
					StateUpdateWithBlock(gomock.Any(), blockNumberStr).
					Return(emptyStateUpdate(), nil)
				mockClient.EXPECT().
					Signature(gomock.Any(), blockNumberStr).
					Return(emptySignature(), nil)

				su, blk, err := fa.StateUpdateWithBlock(t.Context(), blockNumber)
				require.NoError(t, err)
				require.NotNil(t, su)
				require.NotNil(t, blk)

				// ************************************************/
				// ****** starting verification loop service. *****/
				// ************************************************/

				// The first feeder call for the new endpoint starts immediately.
				// Here, we simulate a failure, meaning the feeder is not updated yet.
				mockClient.EXPECT().
					StateUpdateWithBlockAndSignature(gomock.Any(), latestID).
					Return(nil, errors.New("mock error"))

				ctx, cancel := context.WithCancel(t.Context())
				done := make(chan error, 1) // it will be used later in the test
				go func() { done <- fa.Run(ctx) }()
				synctest.Wait()

				// Since the feeder is not updated, the code will fall back to the legacy two-call path.
				mockClient.EXPECT().
					StateUpdateWithBlock(gomock.Any(), blockNumberStr).
					Return(emptyStateUpdate(), nil)
				mockClient.EXPECT().
					Signature(gomock.Any(), blockNumberStr).
					Return(emptySignature(), nil)

				su, blk, err = fa.StateUpdateWithBlock(t.Context(), blockNumber)
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

				su, blk, err = fa.StateUpdateWithBlock(t.Context(), blockNumber)
				require.NoError(t, err)
				require.NotNil(t, su)
				require.NotNil(t, blk)

				// **************************************************************/
				// ******* 1st verification tick, feeder is not updated yet *****/
				// **************************************************************/
				mockClient.EXPECT().
					StateUpdateWithBlockAndSignature(gomock.Any(), latestID).
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

				su, blk, err = fa.StateUpdateWithBlock(t.Context(), blockNumber)
				require.NoError(t, err)
				require.NotNil(t, su)
				require.NotNil(t, blk)

				// ******************************************************/
				// ******* 2nd verification tick, feeder is updated *****/
				// ******************************************************/
				mockClient.EXPECT().
					StateUpdateWithBlockAndSignature(gomock.Any(), latestID).
					Return(emptyStateUpdateWithSig(), nil)

				time.Sleep(verificationInterval)
				synctest.Wait()

				// The new endpoint is being used.
				mockClient.EXPECT().
					StateUpdateWithBlockAndSignature(gomock.Any(), blockNumberStr).
					Return(emptyStateUpdateWithSig(), nil)

				su, blk, err = fa.StateUpdateWithBlock(t.Context(), blockNumber)
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
		})

	t.Run("feeder is already updated when the verification loop starts", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := mocks.NewMockFeederReader(ctrl)

			af := adaptfeeder.NewFeederAdapter(adaptfeeder.New(mockClient), log.NewNopZapLogger())

			// The first feeder call for the new endpoint starts immediately.
			// By returning a valid response, we simulate that the feeder is already updated.
			mockClient.EXPECT().
				StateUpdateWithBlockAndSignature(gomock.Any(), latestID).
				Return(emptyStateUpdateWithSig(), nil)

			go func() { _ = af.Run(t.Context()) }()
			synctest.Wait()

			// The new endpoint is used.
			mockClient.EXPECT().
				StateUpdateWithBlockAndSignature(gomock.Any(), blockNumberStr).
				Return(emptyStateUpdateWithSig(), nil)

			su, blk, err := af.StateUpdateWithBlock(t.Context(), blockNumber)
			require.NoError(t, err)
			require.NotNil(t, su)
			require.NotNil(t, blk)
		})
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

func TestFeederAdapter_PreConfirmedBlockByNumber(t *testing.T) {
	blockNumber := uint64(1234)
	blockNumberStr := strconv.FormatUint(blockNumber, 10)

	t.Run(
		"start with an outdated feeder and it gets updated afterwards",
		func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				mockClient := mocks.NewMockFeederReader(ctrl)

				fa := adaptfeeder.NewFeederAdapter(adaptfeeder.New(mockClient), log.NewNopZapLogger())

				// ************************************************/
				// ***** uses legacy path by default *****/
				// ************************************************/
				mockClient.EXPECT().
					DeprecatedPreConfirmedBlock(gomock.Any(), blockNumberStr).
					Return(emptyPreConfirmed(), nil)

				pblock, err := fa.PreConfirmedBlockByNumber(t.Context(), blockNumber, "", 0)
				require.NoError(t, err)
				require.NotNil(t, pblock)

				// ************************************************/
				// ****** starting verification loop service. *****/
				// ************************************************/

				// The first feeder call for the new endpoint starts immediately.
				// Here, we simulate a failure, meaning the feeder is not updated yet.
				mockClient.EXPECT().
					StateUpdateWithBlockAndSignature(gomock.Any(), latestID).
					Return(nil, errors.New("mock error"))

				go func() { _ = fa.Run(t.Context()) }()
				synctest.Wait()

				// Since the feeder is not updated, the code will fall back to the legacy path.
				mockClient.EXPECT().
					DeprecatedPreConfirmedBlock(gomock.Any(), blockNumberStr).
					Return(emptyPreConfirmed(), nil)

				pblock, err = fa.PreConfirmedBlockByNumber(t.Context(), blockNumber, "", 0)
				require.NoError(t, err)
				require.NotNil(t, pblock)

				// **************************************************************/
				// ******* 1st verification tick, feeder is not updated yet *****/
				// **************************************************************/
				mockClient.EXPECT().
					StateUpdateWithBlockAndSignature(gomock.Any(), latestID).
					Return(nil, errors.New("mock error"))
				time.Sleep(verificationInterval)
				synctest.Wait()

				// The legacy two-call path is still being used.
				mockClient.EXPECT().
					DeprecatedPreConfirmedBlock(gomock.Any(), blockNumberStr).
					Return(emptyPreConfirmed(), nil)

				pblock, err = fa.PreConfirmedBlockByNumber(t.Context(), blockNumber, "", 0)
				require.NoError(t, err)
				require.NotNil(t, pblock)

				// ******************************************************/
				// ******* 2nd verification tick, feeder is updated *****/
				// ******************************************************/
				mockClient.EXPECT().
					StateUpdateWithBlockAndSignature(gomock.Any(), latestID).
					Return(emptyStateUpdateWithSig(), nil)

				time.Sleep(verificationInterval)
				synctest.Wait()

				// The new endpoint is being used.
				mockClient.EXPECT().
					PreConfirmedBlockWithIdentifier(gomock.Any(), blockNumberStr, "", uint64(0)).
					Return(emptyPreConfirmed().AsUpdate(), nil)

				pblock, err = fa.PreConfirmedBlockByNumber(t.Context(), blockNumber, "", 0)
				require.NoError(t, err)
				require.NotNil(t, pblock)
			})
		})

	t.Run("feeder is already updated when the verification loop starts", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := mocks.NewMockFeederReader(ctrl)

			fa := adaptfeeder.NewFeederAdapter(adaptfeeder.New(mockClient), log.NewNopZapLogger())

			// The first feeder call for the new endpoint starts immediately.
			// By returning a valid response, we simulate that the feeder is already updated.
			mockClient.EXPECT().
				StateUpdateWithBlockAndSignature(gomock.Any(), latestID).
				Return(emptyStateUpdateWithSig(), nil)

			go func() { _ = fa.Run(t.Context()) }()
			synctest.Wait()

			// The new endpoint is used.
			mockClient.EXPECT().
				PreConfirmedBlockWithIdentifier(gomock.Any(), blockNumberStr, "", uint64(0)).
				Return(emptyPreConfirmed().AsUpdate(), nil)

			pblock, err := fa.PreConfirmedBlockByNumber(t.Context(), blockNumber, "", 0)
			require.NoError(t, err)
			require.NotNil(t, pblock)
		})
	})
}

//nolint:staticcheck // legacy fixture intentionally uses the deprecated type
func emptyPreConfirmed() *starknet.DeprecatedPreConfirmedBlock {
	return &starknet.DeprecatedPreConfirmedBlock{
		Transactions:          []starknet.Transaction{},
		Receipts:              []*starknet.TransactionReceipt{},
		TransactionStateDiffs: []*starknet.StateDiff{},
		Status:                "PRE_CONFIRMED",
		Timestamp:             1234567890,
		Version:               "0.14.2",
		SequencerAddress:      &felt.Zero,
		L1GasPrice:            &starknet.GasPrice{},
		L2GasPrice:            &starknet.GasPrice{},
		L1DAMode:              starknet.L1DAMode(0),
		L1DataGasPrice:        &starknet.GasPrice{},
	}
}

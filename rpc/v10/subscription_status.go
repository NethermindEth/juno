package rpcv10

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/sync"
)

type errorTxnHashNotFound struct {
	txHash felt.Felt
}

func (e errorTxnHashNotFound) Error() string {
	return fmt.Sprintf("transaction %v not found", e.txHash)
}

// The function signature of SubscribeTransactionStatus cannot be changed
// since the jsonrpc package maps the number
// of argument in the function to the parameters in the starknet spec,
// therefore, the following variables are not passed
// as arguments, and they can be modified in the test to make them run faster.
var (
	subscribeTxStatusTimeout        = 5 * time.Minute
	subscribeTxStatusTickerDuration = time.Second
)

// SubscribeTransactionStatus subscribes to status changes of a transaction.
// It checks for updates each time a new block is added.
// Later updates are sent only when the transaction status changes.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/785257f27cdc4ea0ca3b62a21b0f7bf51000f9b1/api/starknet_ws_api.json#L151
//
//nolint:lll,nolintlint // url exceeds line limit, nolintlint because conflicting line limit with other lint rules
func (h *Handler) SubscribeTransactionStatus(
	ctx context.Context,
	txHash *felt.Felt,
) (SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return "", jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	return h.subscribe(ctx, w, newTxStatusSubscriber(h, w, txHash))
}

type txStatusSubscriberState struct {
	handler    *Handler
	conn       jsonrpc.Conn
	txHash     *felt.Felt
	lastStatus TxnStatus
}

func newTxStatusSubscriber(h *Handler, w jsonrpc.Conn, txHash *felt.Felt) subscriber {
	state := &txStatusSubscriberState{
		handler: h,
		conn:    w,
		txHash:  txHash,
	}

	return subscriber{
		onStart:       state.onStart,
		onReorg:       state.onReorg,
		onNewHead:     state.onNewHead,
		onPreLatest:   state.onPreLatest,
		onPendingData: state.onPendingData,
		onL1Head:      state.onL1Head,
	}
}

func (s *txStatusSubscriberState) onStart(
	ctx context.Context,
	id string,
	sub *subscription,
	_ any,
) error {
	return s.getInitialTxStatus(ctx, id, sub)
}

func (s *txStatusSubscriberState) onReorg(
	_ context.Context,
	id string,
	_ *subscription,
	reorg *sync.ReorgBlockRange,
) error {
	return sendReorg(s.conn, reorg, id)
}

func (s *txStatusSubscriberState) onNewHead(
	ctx context.Context,
	id string,
	sub *subscription,
	_ *core.Block,
) error {
	return s.checkTxStatusIfPending(ctx, id, sub)
}

func (s *txStatusSubscriberState) onPreLatest(
	ctx context.Context,
	id string,
	sub *subscription,
	_ *core.PreLatest,
) error {
	return s.checkTxStatusIfPending(ctx, id, sub)
}

func (s *txStatusSubscriberState) onPendingData(
	ctx context.Context,
	id string,
	sub *subscription,
	_ core.PendingData,
) error {
	return s.checkTxStatusIfPending(ctx, id, sub)
}

func (s *txStatusSubscriberState) onL1Head(
	ctx context.Context,
	id string,
	sub *subscription,
	_ *core.L1Head,
) error {
	return s.checkTxStatus(ctx, id, sub)
}

// If the error is transaction not found that means the transaction has not
// been submitted to the feeder gateway, therefore, we need to wait for a
// specified time and at regular interval check if the transaction has been found.
// If the transaction is found during the timeout expiry,
// then we continue to keep track of its status otherwise the
// websocket connection is closed after the expiry.
func (s *txStatusSubscriberState) getInitialTxStatus(
	ctx context.Context,
	id string,
	sub *subscription,
) error {
	err := s.checkTxStatus(ctx, id, sub)
	if !errors.Is(err, errorTxnHashNotFound{*s.txHash}) {
		return err
	}

	ctx, cancelTimeout := context.WithTimeout(ctx, subscribeTxStatusTimeout)
	defer cancelTimeout()
	ticker := time.Tick(subscribeTxStatusTickerDuration)

	for {
		select {
		case <-ctx.Done():
			return err
		case <-ticker:
			err = s.checkTxStatus(ctx, id, sub)
			if !errors.Is(err, errorTxnHashNotFound{*s.txHash}) {
				return err
			}
		}
	}
}

// checkTxStatusIfPending checks the transaction status only if the last known status
// is less than TxnStatusAcceptedOnL2 (i.e., the transaction is still pending).
// If the transaction has already reached or surpassed TxnStatusAcceptedOnL2,
// it returns the last known status without making an additional status check.
func (s *txStatusSubscriberState) checkTxStatusIfPending(
	ctx context.Context,
	id string,
	sub *subscription,
) error {
	if s.lastStatus < TxnStatusAcceptedOnL2 {
		return s.checkTxStatus(ctx, id, sub)
	}
	return nil
}

func (s *txStatusSubscriberState) checkTxStatus(
	ctx context.Context,
	id string,
	sub *subscription,
) error {
	status, rpcErr := s.handler.TransactionStatus(ctx, s.txHash)
	if rpcErr != nil {
		if rpcErr != rpccore.ErrTxnHashNotFound {
			return fmt.Errorf(
				"error while checking status for transaction %v with rpc error message: %v",
				s.txHash,
				rpcErr.Message,
			)
		}
		return errorTxnHashNotFound{*s.txHash}
	}

	if status.Finality == s.lastStatus {
		return nil
	}

	err := sendTxnStatus(
		s.conn,
		SubscriptionTransactionStatus{
			TransactionHash: s.txHash,
			Status:          status,
		},
		id,
	)
	if err != nil {
		return err
	}

	if status.Finality == TxnStatusAcceptedOnL1 {
		sub.cancel()
	}

	s.lastStatus = status.Finality
	return nil
}

func sendTxnStatus(w jsonrpc.Conn, status SubscriptionTransactionStatus, id string) error {
	return sendResponse("starknet_subscriptionTransactionStatus", w, id, status)
}

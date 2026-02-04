package rpcv7

import (
	"context"
	"encoding/json"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

type Event struct {
	From *felt.Felt   `json:"from_address,omitempty"`
	Keys []*felt.Felt `json:"keys"`
	Data []*felt.Felt `json:"data"`
}

/****************************************************
		Events Handlers
*****************************************************/

func (h *Handler) SubscribeNewHeads(ctx context.Context) (uint64, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return 0, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	id := h.idgen()
	subscriptionCtx, subscriptionCtxCancel := context.WithCancel(ctx)
	sub := &subscription{
		cancel: subscriptionCtxCancel,
		conn:   w,
	}
	headerSub := h.newHeads.Subscribe()
	sub.wg.Go(func() {
		defer func() {
			headerSub.Unsubscribe()
			h.unsubscribe(sub, id)
		}()
		for {
			select {
			case <-subscriptionCtx.Done():
				return
			case header := <-headerSub.Recv():
				resp, err := json.Marshal(jsonrpc.Request{
					Version: "2.0",
					Method:  "juno_subscribeNewHeads",
					Params: map[string]any{
						"result":       adaptBlockHeader(header.Header),
						"subscription": id,
					},
				})
				if err != nil {
					h.log.Warn("Error marshalling a subscription reply", utils.SugaredFields("err", err)...)
					return
				}
				if _, err = w.Write(resp); err != nil {
					h.log.Warn("Error writing a subscription reply", utils.SugaredFields("err", err)...)
					return
				}
			}
		}
	})
	return id, nil
}

func (h *Handler) Unsubscribe(ctx context.Context, id uint64) (bool, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return false, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}
	sub, ok := h.subscriptions.Load(id)
	if !ok {
		return false, rpccore.ErrInvalidSubscriptionID
	}

	subs := sub.(*subscription)
	if !subs.conn.Equal(w) {
		return false, rpccore.ErrInvalidSubscriptionID
	}

	subs.cancel()
	subs.wg.Wait() // Let the subscription finish before responding.
	h.subscriptions.Delete(id)
	return true, nil
}

// unsubscribe assumes h.mu is unlocked. It releases all subscription resources.
func (h *Handler) unsubscribe(sub *subscription, id uint64) {
	sub.cancel()
	h.subscriptions.Delete(id)
}

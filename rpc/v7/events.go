package rpcv7

import (
	"context"
	"encoding/json"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
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
					h.log.Warnw("Error marshalling a subscription reply", "err", err)
					return
				}
				if _, err = w.Write(resp); err != nil {
					h.log.Warnw("Error writing a subscription reply", "err", err)
					return
				}
			}
		}
	})
	return id, nil
}

// unsubscribe assumes h.mu is unlocked. It releases all subscription resources.
func (h *Handler) unsubscribe(sub *subscription, id uint64) {
	sub.cancel()
	h.subscriptions.Delete(id)
}

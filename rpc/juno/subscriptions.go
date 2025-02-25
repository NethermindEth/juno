package juno

import (
	"context"
	"encoding/json"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/jsonrpc"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
)

func (a v6Adapter) AdaptBlockHeader(header *core.Header) rpcv6.BlockHeader {
	return rpcv6.AdaptBlockHeader(header)
}

func (a v7Adapter) AdaptBlockHeader(header *core.Header) rpcv6.BlockHeader {
	return rpcv7.AdaptBlockHeader(header)
}

func (h *Handler) SubscribeNewHeads(adapter Adapter) func(ctx context.Context) (uint64, *jsonrpc.Error) {
	return func(ctx context.Context) (uint64, *jsonrpc.Error) {
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
							"result":       adapter.AdaptBlockHeader(header),
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

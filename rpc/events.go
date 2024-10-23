package rpc

import (
	"context"
	"encoding/json"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/sync"
	"github.com/sourcegraph/conc"
)

const (
	MaxBlocksBack        = 1024
	MaxAddressesInFilter = 1000 // TODO(weiihann): not finalised yet
)

type EventsArg struct {
	EventFilter
	ResultPageRequest
}

type SubscriptionID struct {
	ID uint64 `json:"subscription_id"`
}

type EventFilter struct {
	FromBlock *BlockID      `json:"from_block"`
	ToBlock   *BlockID      `json:"to_block"`
	Address   *felt.Felt    `json:"address"`
	Keys      [][]felt.Felt `json:"keys"`
}

type ResultPageRequest struct {
	ContinuationToken string `json:"continuation_token"`
	ChunkSize         uint64 `json:"chunk_size" validate:"min=1"`
}

type Event struct {
	From *felt.Felt   `json:"from_address,omitempty"`
	Keys []*felt.Felt `json:"keys"`
	Data []*felt.Felt `json:"data"`
}

type EmittedEvent struct {
	*Event
	BlockNumber     *uint64    `json:"block_number,omitempty"`
	BlockHash       *felt.Felt `json:"block_hash,omitempty"`
	TransactionHash *felt.Felt `json:"transaction_hash"`
}

type EventsChunk struct {
	Events            []*EmittedEvent `json:"events"`
	ContinuationToken string          `json:"continuation_token,omitempty"`
}

/****************************************************
		Events Handlers
*****************************************************/

func (h *Handler) SubscribeNewHeads(ctx context.Context, blockID *BlockID) (*SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return nil, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	startHeader, latestHeader, rpcErr := h.getStartAndLatestHeaders(blockID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	id := h.idgen()
	subscriptionCtx, subscriptionCtxCancel := context.WithCancel(ctx)
	sub := &subscription{
		cancel: subscriptionCtxCancel,
		conn:   w,
	}
	h.mu.Lock()
	h.subscriptions[id] = sub
	h.mu.Unlock()

	headerSub := h.newHeads.Subscribe()
	reorgSub := h.reorgs.Subscribe() // as per the spec, reorgs are also sent in the new heads subscription
	sub.wg.Go(func() {
		defer func() {
			h.unsubscribe(sub, id)
			headerSub.Unsubscribe()
			reorgSub.Unsubscribe()
		}()

		var wg conc.WaitGroup

		newHeadersChan := make(chan *core.Header, MaxBlocksBack)
		wg.Go(func() {
			h.bufferNewHeaders(subscriptionCtx, headerSub, newHeadersChan)
		})

		if err := h.sendHistoricalHeaders(subscriptionCtx, startHeader, latestHeader, w, id); err != nil {
			h.log.Errorw("Error sending old headers", "err", err)
			return
		}

		wg.Go(func() {
			h.processNewHeaders(subscriptionCtx, newHeadersChan, w, id)
		})

		wg.Go(func() {
			h.processReorgs(subscriptionCtx, reorgSub, w, id)
		})

		wg.Wait()
	})

	return &SubscriptionID{ID: id}, nil
}

func (h *Handler) SubscribePendingTxs(ctx context.Context, getDetails *bool, senderAddr []felt.Felt) (*SubscriptionID, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return nil, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	if len(senderAddr) > MaxAddressesInFilter {
		return nil, ErrTooManyAddressesInFilter
	}

	id := h.idgen()
	subscriptionCtx, subscriptionCtxCancel := context.WithCancel(ctx)
	sub := &subscription{
		cancel: subscriptionCtxCancel,
		conn:   w,
	}
	h.mu.Lock()
	h.subscriptions[id] = sub
	h.mu.Unlock()

	pendingTxsSub := h.pendingTxs.Subscribe()
	sub.wg.Go(func() {
		defer func() {
			h.unsubscribe(sub, id)
			pendingTxsSub.Unsubscribe()
		}()

		h.processPendingTxs(subscriptionCtx, getDetails != nil && *getDetails, senderAddr, pendingTxsSub, w, id)
	})

	return &SubscriptionID{ID: id}, nil
}

func (h *Handler) processPendingTxs(
	ctx context.Context,
	getDetails bool,
	senderAddr []felt.Felt,
	pendingTxsSub *feed.Subscription[[]core.Transaction],
	w jsonrpc.Conn,
	id uint64,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case pendingTxs := <-pendingTxsSub.Recv():
			filteredTxs := h.filterTxs(pendingTxs, getDetails, senderAddr)
			if err := h.sendPendingTxs(w, filteredTxs, id); err != nil {
				h.log.Warnw("Error sending pending transactions", "err", err)
				return
			}
		}
	}
}

func (h *Handler) filterTxs(pendingTxs []core.Transaction, getDetails bool, senderAddr []felt.Felt) interface{} {
	if getDetails {
		return h.filterTxDetails(pendingTxs, senderAddr)
	}
	return h.filterTxHashes(pendingTxs, senderAddr)
}

func (h *Handler) filterTxDetails(pendingTxs []core.Transaction, senderAddr []felt.Felt) []*Transaction {
	filteredTxs := make([]*Transaction, 0, len(pendingTxs))
	for _, txn := range pendingTxs {
		if h.shouldIncludeTx(txn, senderAddr) {
			filteredTxs = append(filteredTxs, AdaptTransaction(txn))
		}
	}
	return filteredTxs
}

func (h *Handler) filterTxHashes(pendingTxs []core.Transaction, senderAddr []felt.Felt) []felt.Felt {
	filteredTxHashes := make([]felt.Felt, 0, len(pendingTxs))
	for _, txn := range pendingTxs {
		if h.shouldIncludeTx(txn, senderAddr) {
			filteredTxHashes = append(filteredTxHashes, *txn.Hash())
		}
	}
	return filteredTxHashes
}

func (h *Handler) shouldIncludeTx(txn core.Transaction, senderAddr []felt.Felt) bool {
	if len(senderAddr) == 0 {
		return true
	}

	//
	switch t := txn.(type) {
	case *core.InvokeTransaction:
		for _, addr := range senderAddr {
			if t.SenderAddress.Equal(&addr) {
				return true
			}
		}
	case *core.DeclareTransaction:
		for _, addr := range senderAddr {
			if t.SenderAddress.Equal(&addr) {
				return true
			}
		}
	}

	return false
}

func (h *Handler) sendPendingTxs(w jsonrpc.Conn, result interface{}, id uint64) error {
	req := jsonrpc.Request{
		Version: "2.0",
		Method:  "starknet_subscriptionPendingTransactions",
		Params: map[string]interface{}{
			"subscription_id": id,
			"result":          result,
		},
	}

	resp, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = w.Write(resp)
	return err
}

// getStartAndLatestHeaders gets the start and latest header for the subscription
func (h *Handler) getStartAndLatestHeaders(blockID *BlockID) (*core.Header, *core.Header, *jsonrpc.Error) {
	if blockID == nil || blockID.Latest {
		return nil, nil, nil
	}

	latestHeader, err := h.bcReader.HeadsHeader()
	if err != nil {
		return nil, nil, ErrInternal
	}

	startHeader, rpcErr := h.blockHeaderByID(blockID)
	if rpcErr != nil {
		return nil, nil, rpcErr
	}

	if latestHeader.Number > MaxBlocksBack && startHeader.Number < latestHeader.Number-MaxBlocksBack {
		return nil, nil, ErrTooManyBlocksBack
	}

	return startHeader, latestHeader, nil
}

// sendHistoricalHeaders sends a range of headers from the start header until the latest header
func (h *Handler) sendHistoricalHeaders(
	ctx context.Context,
	startHeader *core.Header,
	latestHeader *core.Header,
	w jsonrpc.Conn,
	id uint64,
) error {
	if startHeader == nil {
		return nil
	}

	var err error

	lastHeader := startHeader
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := h.sendHeader(w, lastHeader, id); err != nil {
				return err
			}

			if lastHeader.Number == latestHeader.Number {
				return nil
			}

			lastHeader, err = h.bcReader.BlockHeaderByNumber(lastHeader.Number + 1)
			if err != nil {
				return err
			}
		}
	}
}

func (h *Handler) bufferNewHeaders(ctx context.Context, headerSub *feed.Subscription[*core.Header], newHeadersChan chan<- *core.Header) {
	for {
		select {
		case <-ctx.Done():
			return
		case header := <-headerSub.Recv():
			newHeadersChan <- header
		}
	}
}

func (h *Handler) processNewHeaders(ctx context.Context, newHeadersChan <-chan *core.Header, w jsonrpc.Conn, id uint64) {
	for {
		select {
		case <-ctx.Done():
			return
		case header := <-newHeadersChan:
			if err := h.sendHeader(w, header, id); err != nil {
				h.log.Warnw("Error sending header", "err", err)
				return
			}
		}
	}
}

// sendHeader creates a request and sends it to the client
func (h *Handler) sendHeader(w jsonrpc.Conn, header *core.Header, id uint64) error {
	resp, err := json.Marshal(jsonrpc.Request{
		Version: "2.0",
		Method:  "starknet_subscriptionNewHeads",
		Params: map[string]any{
			"subscription_id": id,
			"result":          adaptBlockHeader(header),
		},
	})
	if err != nil {
		return err
	}
	_, err = w.Write(resp)
	return err
}

func (h *Handler) processReorgs(ctx context.Context, reorgSub *feed.Subscription[*sync.ReorgData], w jsonrpc.Conn, id uint64) {
	for {
		select {
		case <-ctx.Done():
			return
		case reorg := <-reorgSub.Recv():
			if err := h.sendReorg(w, reorg, id); err != nil {
				h.log.Warnw("Error sending reorg", "err", err)
				return
			}
		}
	}
}

func (h *Handler) sendReorg(w jsonrpc.Conn, reorg *sync.ReorgData, id uint64) error {
	resp, err := json.Marshal(jsonrpc.Request{
		Version: "2.0",
		Method:  "starknet_subscriptionReorg",
		Params: map[string]any{
			"subscription_id": id,
			"result":          reorg,
		},
	})
	if err != nil {
		return err
	}
	_, err = w.Write(resp)
	return err
}

func (h *Handler) SubscribeTxnStatus(ctx context.Context, txHash felt.Felt, _ *BlockID) (*SubscriptionID, *jsonrpc.Error) {
	var (
		lastKnownStatus, lastSendStatus *TransactionStatus
		wrapResult                      = func(s *TransactionStatus) *NewTransactionStatus {
			return &NewTransactionStatus{
				TransactionHash: &txHash,
				Status:          s,
			}
		}
	)

	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return nil, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	id := h.idgen()
	subscriptionCtx, subscriptionCtxCancel := context.WithCancel(ctx)
	sub := &subscription{
		cancel: subscriptionCtxCancel,
		conn:   w,
	}

	lastKnownStatus, rpcErr := h.TransactionStatus(subscriptionCtx, txHash)
	if rpcErr != nil {
		h.log.Warnw("Failed to get Tx status", "txHash", &txHash, "rpcErr", rpcErr)
		return nil, rpcErr
	}

	h.mu.Lock()
	h.subscriptions[id] = sub
	h.mu.Unlock()

	statusSub := h.txnStatus.Subscribe()
	headerSub := h.newHeads.Subscribe()
	sub.wg.Go(func() {
		defer func() {
			h.unsubscribe(sub, id)
			statusSub.Unsubscribe()
			headerSub.Unsubscribe()
		}()

		if err := h.sendTxnStatus(sub.conn, wrapResult(lastKnownStatus), id); err != nil {
			h.log.Warnw("Error while sending Txn status", "txHash", txHash)
			return
		}
		lastSendStatus = lastKnownStatus

		for {
			select {
			case <-subscriptionCtx.Done():
				return
			case <-headerSub.Recv():
				lastKnownStatus, rpcErr = h.TransactionStatus(subscriptionCtx, txHash)
				if rpcErr != nil {
					h.log.Warnw("Failed to get Tx status", "txHash", txHash, "rpcErr", rpcErr)
					return
				}

				if *lastKnownStatus != *lastSendStatus {
					if err := h.sendTxnStatus(sub.conn, wrapResult(lastKnownStatus), id); err != nil {
						h.log.Warnw("Error while sending Txn status", "txHash", txHash)
						return
					}
					lastSendStatus = lastKnownStatus
				}

				// Stop when final status reached and notified
				if isFinal(lastSendStatus) {
					return
				}
			}
		}
	})

	return &SubscriptionID{ID: id}, nil
}

func (h *Handler) Unsubscribe(ctx context.Context, id uint64) (bool, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return false, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}
	h.mu.Lock()
	sub, ok := h.subscriptions[id]
	h.mu.Unlock() // Don't defer since h.unsubscribe acquires the lock.
	if !ok || !sub.conn.Equal(w) {
		return false, ErrSubscriptionNotFound
	}
	sub.cancel()
	sub.wg.Wait() // Let the subscription finish before responding.
	return true, nil
}

// Events gets the events matching a filter
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/94a969751b31f5d3e25a0c6850c723ddadeeb679/api/starknet_api_openrpc.json#L642
func (h *Handler) Events(args EventsArg) (*EventsChunk, *jsonrpc.Error) {
	if args.ChunkSize > maxEventChunkSize {
		return nil, ErrPageSizeTooBig
	} else {
		lenKeys := len(args.Keys)
		for _, keys := range args.Keys {
			lenKeys += len(keys)
		}
		if lenKeys > maxEventFilterKeys {
			return nil, ErrTooManyKeysInFilter
		}
	}

	height, err := h.bcReader.Height()
	if err != nil {
		return nil, ErrInternal
	}

	filter, err := h.bcReader.EventFilter(args.EventFilter.Address, args.EventFilter.Keys)
	if err != nil {
		return nil, ErrInternal
	}
	filter = filter.WithLimit(h.filterLimit)
	defer h.callAndLogErr(filter.Close, "Error closing event filter in events")

	var cToken *blockchain.ContinuationToken
	if args.ContinuationToken != "" {
		cToken = new(blockchain.ContinuationToken)
		if err = cToken.FromString(args.ContinuationToken); err != nil {
			return nil, ErrInvalidContinuationToken
		}
	}

	if err = setEventFilterRange(filter, args.EventFilter.FromBlock, args.EventFilter.ToBlock, height); err != nil {
		return nil, ErrBlockNotFound
	}

	filteredEvents, cToken, err := filter.Events(cToken, args.ChunkSize)
	if err != nil {
		return nil, ErrInternal
	}

	emittedEvents := make([]*EmittedEvent, 0, len(filteredEvents))
	for _, fEvent := range filteredEvents {
		var blockNumber *uint64
		if fEvent.BlockHash != nil {
			blockNumber = &(fEvent.BlockNumber)
		}
		emittedEvents = append(emittedEvents, &EmittedEvent{
			BlockNumber:     blockNumber,
			BlockHash:       fEvent.BlockHash,
			TransactionHash: fEvent.TransactionHash,
			Event: &Event{
				From: fEvent.From,
				Keys: fEvent.Keys,
				Data: fEvent.Data,
			},
		})
	}

	cTokenStr := ""
	if cToken != nil {
		cTokenStr = cToken.String()
	}
	return &EventsChunk{Events: emittedEvents, ContinuationToken: cTokenStr}, nil
}

// unsubscribe assumes h.mu is unlocked. It releases all subscription resources.
func (h *Handler) unsubscribe(sub *subscription, id uint64) {
	sub.cancel()
	h.mu.Lock()
	delete(h.subscriptions, id)
	h.mu.Unlock()
}

func setEventFilterRange(filter *blockchain.EventFilter, fromID, toID *BlockID, latestHeight uint64) error {
	set := func(filterRange blockchain.EventFilterRange, id *BlockID) error {
		if id == nil {
			return nil
		}

		switch {
		case id.Latest:
			return filter.SetRangeEndBlockByNumber(filterRange, latestHeight)
		case id.Hash != nil:
			return filter.SetRangeEndBlockByHash(filterRange, id.Hash)
		case id.Pending:
			return filter.SetRangeEndBlockByNumber(filterRange, latestHeight+1)
		default:
			return filter.SetRangeEndBlockByNumber(filterRange, id.Number)
		}
	}
	if err := set(blockchain.EventFilterFrom, fromID); err != nil {
		return err
	}
	return set(blockchain.EventFilterTo, toID)
}

type NewTransactionStatus struct {
	TransactionHash *felt.Felt         `json:"transaction_hash"`
	Status          *TransactionStatus `json:"status"`
}

// sendHeader creates a request and sends it to the client
func (h *Handler) sendTxnStatus(w jsonrpc.Conn, status *NewTransactionStatus, id uint64) error {
	resp, err := json.Marshal(jsonrpc.Request{
		Version: "2.0",
		Method:  "starknet_subscriptionTransactionsStatus",
		Params: map[string]any{
			"subscription_id": id,
			"result":          status,
		},
	})
	if err != nil {
		return err
	}
	h.log.Debugw("Sending Txn status", "status", string(resp))
	_, err = w.Write(resp)
	return err
}

func isFinal(status *TransactionStatus) bool {
	return status.Finality == TxnStatusRejected || status.Finality == TxnStatusAcceptedOnL1
}

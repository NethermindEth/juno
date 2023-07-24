package jsonrpc

import (
	"context"
	"encoding/json"
	"io"
)

type subscriptionServerKey struct{}

type SubscriptionServer struct {
	conn       io.ReadWriter
	id         uint64
	methodName string
}

func SubscriptionServerFromContext(ctx context.Context) *SubscriptionServer {
	subscriptionServer, ok := ctx.Value(subscriptionServerKey{}).(*SubscriptionServer)
	if !ok {
		panic("subscriptions are not supported on this handler")
	}
	return subscriptionServer
}

type subscriptionResult struct {
	Subscription uint64 `json:"subscription"`
	Result       any    `json:"result"`
}

func (s *SubscriptionServer) Send(data any) error {
	resp, err := json.Marshal(&request{
		Version: "2.0",
		Method:  s.methodName,
		Params: &subscriptionResult{
			Subscription: s.id,
			Result:       data,
		},
	})
	if err != nil {
		return err
	}
	_, err = s.conn.Write(resp)
	return err
}

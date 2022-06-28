package starknet

import "context"

type StarkNetRpc struct{}

func (s *StarkNetRpc) Ping(ctx context.Context) (interface{}, error) {
	return "Pong", nil
}

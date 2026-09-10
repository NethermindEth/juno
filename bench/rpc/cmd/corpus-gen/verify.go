package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

type verifyFlag bool

func (f *verifyFlag) bind(cmd *cobra.Command, _ *rpcClient) {
	cmd.Flags().BoolVar(
		(*bool)(f),
		"verify",
		true,
		"Replay each sampled request against the source node and draw again when it fails.",
	)
}

// verify replays params against the source node, and asks for another draw when
// the node rejects them; k6 counts json-rpc errors as failures, so a request
// that reverts or runs out of resources would spoil the benchmark.
func verify(ctx context.Context, client *rpcClient, method string, params any) error {
	if _, err := client.rpcCall[json.RawMessage](ctx, method, params); err != nil {
		var rpcErr *rpcError
		// Starknet codes are positive; the reserved negative range reports a
		// malformed request, which another draw cannot fix.
		if errors.As(err, &rpcErr) && rpcErr.Code > 0 {
			return fmt.Errorf("%w: %w", err, errResample)
		}
		return err
	}
	return nil
}

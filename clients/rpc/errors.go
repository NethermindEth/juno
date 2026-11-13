package rpc

import (
	"errors"
	"strconv"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
)

// These answers describe the remote chain, so retrying can't change them: the block fetch
// polls past the tip, and a remote with no blocks has no head yet.
func retryable(err error) bool {
	return !IsError(err, rpccore.ErrBlockNotFound) && !IsError(err, rpccore.ErrNoBlock)
}

func IsError(err error, want *jsonrpc.Error) bool {
	code, ok := errorCode(err)
	return ok && code == want.Code
}

func outcome(err error) string {
	if err == nil {
		return "ok"
	}
	if code, ok := errorCode(err); ok {
		return strconv.Itoa(code)
	}
	return "error"
}

func errorCode(err error) (int, bool) {
	var rpcErr gethrpc.Error
	if !errors.As(err, &rpcErr) {
		return 0, false
	}
	return rpcErr.ErrorCode(), true
}

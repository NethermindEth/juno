package starknetrpc

import (
	"errors"
	"strconv"

	"github.com/NethermindEth/juno/rpc/rpccore"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
)

// The block fetch polls past the tip, so this answer paces the caller instead of
// burning the retry budget.
func retryable(err error) bool {
	code, isRPCError := errorCode(err)
	return !isRPCError || code != rpccore.ErrBlockNotFound.Code
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

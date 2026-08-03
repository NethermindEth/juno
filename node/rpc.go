package node

import (
	"runtime"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/utils/log"
)

func makeRPCServers(
	cfg *Config,
	rpcHandler *rpc.Handler,
	logger log.StructuredLogger,
) (map[string]*jsonrpc.Server, error) {
	// to improve RPC throughput we double GOMAXPROCS
	maxGoroutines := 2 * runtime.GOMAXPROCS(0)

	jsonrpcServerV10 := jsonrpc.NewServer(maxGoroutines, logger).
		WithValidator(rpcv10.Validator()).
		DisableBatchRequests(cfg.ForbidRPCBatchRequests)
	methodsV10, pathV10 := rpcHandler.MethodsV0_10()
	if err := jsonrpcServerV10.RegisterMethods(methodsV10...); err != nil {
		return nil, err
	}

	jsonrpcServerV09 := jsonrpc.NewServer(maxGoroutines, logger).
		WithValidator(rpcv9.Validator()).
		DisableBatchRequests(cfg.ForbidRPCBatchRequests)
	methodsV09, pathV09 := rpcHandler.MethodsV0_9()
	if err := jsonrpcServerV09.RegisterMethods(methodsV09...); err != nil {
		return nil, err
	}

	jsonrpcServerV08 := jsonrpc.NewServer(maxGoroutines, logger).
		WithValidator(rpcv8.Validator()).
		DisableBatchRequests(cfg.ForbidRPCBatchRequests)
	methodsV08, pathV08 := rpcHandler.MethodsV0_8()
	if err := jsonrpcServerV08.RegisterMethods(methodsV08...); err != nil {
		return nil, err
	}

	if cfg.Metrics {
		rpcMetrics := makeRPCMetrics(pathV10, pathV09, pathV08)
		jsonrpcServerV10.WithListener(rpcMetrics[0])
		jsonrpcServerV09.WithListener(rpcMetrics[1])
		jsonrpcServerV08.WithListener(rpcMetrics[2])
	}

	// All the following endpoints will be available for both HTTP and WS.
	// Also, additional WS endpoints will be created in the following format: /ws/<path>
	// E.g.:
	// /ws + /
	// /ws + /rpc
	// /ws + /v0_10
	// /ws + /rpc/v0_10
	rpcServers := map[string]*jsonrpc.Server{
		// Default RPC endpoints
		"/":    jsonrpcServerV10,
		"/rpc": jsonrpcServerV10,

		pathV10:          jsonrpcServerV10,
		pathV09:          jsonrpcServerV09,
		pathV08:          jsonrpcServerV08,
		"/rpc" + pathV10: jsonrpcServerV10,
		"/rpc" + pathV09: jsonrpcServerV09,
		"/rpc" + pathV08: jsonrpcServerV08,
	}

	return rpcServers, nil
}

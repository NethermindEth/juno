package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	feederPrefix      = "/feeder_gateway/"
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 5 * time.Second

	blockNotFound    = gateway.ErrorCode("StarknetErrorCode.BLOCK_NOT_FOUND")
	malformedRequest = gateway.ErrorCode("StarknetErrorCode.MALFORMED_REQUEST")
)

type responder func(requestURL *url.URL) ([]byte, error)

type server struct {
	store  *store
	clock  *clock
	window *window
	config *config
	logger *log.ZapLogger
}

func (server *server) run(ctx context.Context, listen string) error {
	httpServer := http.Server{
		Addr:              listen,
		Handler:           server.routes(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	})

	group.Go(func() error {
		fields := []zap.Field{
			zap.String("listen", listen),
			zap.Uint64("tip", server.clock.tip()),
			zap.Bool("preconfirmed", server.config.preconfirmed),
		}

		if server.config.preconfirmed {
			fields = append(
				fields,
				zap.Uint64("lead", server.config.lead),
				zap.Uint64("keep", server.config.keep),
				zap.Uint64("stages", server.config.stages),
			)
		}

		server.logger.Info("serving", fields...)

		err := httpServer.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	})

	return group.Wait()
}

func (server *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	server.serve(mux, block, server.checkTip)
	server.serve(mux, stateUpdate, server.checkTip)
	server.serve(mux, classByHash)
	server.serve(mux, compiledClass)
	server.serve(mux, contractAddresses)
	preConfirmedResponder := notInWindow
	if server.config.preconfirmed {
		preConfirmedResponder = server.logged(server.preConfirmedReply)
	}

	mux.Handle(feederPrefix+preConfirmedBlock.name, server.handler(preConfirmedResponder))
	mux.Handle("/", server.handler(server.logged(unknown)))
	return mux
}

func (server *server) handler(respond responder) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		time.Sleep(server.config.latency)

		body, err := respond(request.URL)
		status := http.StatusOK

		var failure gateway.Error
		switch {
		case errors.As(err, &failure):
			status = http.StatusBadRequest
			body, err = json.Marshal(failure)
		case err != nil:
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		case strings.Contains(request.Header.Get("Accept-Encoding"), "gzip"):
			writer.Header().Set("Content-Encoding", "gzip")
		default:
			body, err = gunzip(body)
		}
		if err != nil {
			server.logger.Error("building response", zap.Stringer("url", request.URL), zap.Error(err))
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		if _, err := writer.Write(body); err != nil {
			server.logger.Debug("writing response", zap.Stringer("url", request.URL), zap.Error(err))
		}
	}
}

func (server *server) logged(respond responder) responder {
	return func(requestURL *url.URL) ([]byte, error) {
		body, err := respond(requestURL)
		if err != nil {
			logRejection := server.logger.Error
			var failure gateway.Error
			if errors.As(err, &failure) && failure.Code == blockNotFound {
				logRejection = server.logger.Debug
			}
			logRejection("rejected request", zap.Stringer("url", requestURL), zap.Error(err))
		}
		return body, err
	}
}

func (server *server) serve[K, F comparable](
	mux *http.ServeMux,
	endpoint *endpoint[K, F],
	checks ...func(K) error,
) {
	respond := func(requestURL *url.URL) ([]byte, error) {
		query := requestURL.Query()
		if err := endpoint.checkFixed(query); err != nil {
			return nil, err
		}

		if query.Get("blockNumber") == latestBlock {
			query.Set("blockNumber", strconv.FormatUint(server.clock.tip(), 10))
		}

		key, err := endpoint.key(query)
		if err != nil {
			return nil, err
		}

		for _, check := range checks {
			if err := check(key); err != nil {
				return nil, err
			}
		}

		return server.lookup(endpoint, key)
	}

	mux.Handle(feederPrefix+endpoint.name, server.handler(server.logged(respond)))
}

func unknown(requestURL *url.URL) ([]byte, error) {
	return nil, malformedf("unknown endpoint %s", requestURL.Path)
}

func notInWindow(*url.URL) ([]byte, error) {
	return nil, notFoundf("No pre-confirmed block.")
}

func (server *server) lookup[K, F comparable](endpoint *endpoint[K, F], key K) ([]byte, error) {
	resource, err := endpoint.resource(key)
	if err != nil {
		return nil, err
	}

	body, ok := server.store.get(resource.file)
	if !ok {
		return nil, fmt.Errorf("%s: not in dataset", resource.file)
	}
	return body, nil
}

func (server *server) checkTip(key blockKey) error {
	if key.BlockNumber > server.clock.tip() {
		return notFoundf("Block number %d was not found.", key.BlockNumber)
	}
	if key.BlockNumber < server.config.from {
		return fmt.Errorf(
			"block %d is below --%s %d; Juno's DB is probably not at %d",
			key.BlockNumber, fromFlag, server.config.from, server.config.from-1,
		)
	}
	return nil
}

func notFoundf(format string, args ...any) gateway.Error {
	return gateway.Error{Code: blockNotFound, Message: fmt.Sprintf(format, args...)}
}

func malformedf(format string, args ...any) gateway.Error {
	return gateway.Error{Code: malformedRequest, Message: fmt.Sprintf(format, args...)}
}

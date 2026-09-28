package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type api interface {
	newRequest(ctx context.Context, resource resource) (*http.Request, error)
	decode(dataset dataset, resource resource, body []byte, encoding string) ([]byte, error)
}

type source struct {
	client     *http.Client
	retries    int
	retryDelay time.Duration
	logger     *log.ZapLogger
	api        api
}

type statusError struct {
	code int
}

func (err *statusError) Error() string {
	return fmt.Sprintf("unexpected status %d", err.code)
}

func capture(ctx context.Context, dataset dataset, config *config, logger *log.ZapLogger) error {
	logger.Info(
		"Juno flags for this dataset",
		zap.String("flags", junoFlags(config.network, config.listen, config.preconfirmed)),
	)
	logger.Info(
		"capturing",
		zap.Stringer("network", config.network),
		zap.Uint64("from", config.from),
		zap.Uint64("to", config.to),
	)
	client := newClient(config)
	var preConfirmed fetcher
	if config.preconfirmed {
		preConfirmed = newSource(client, config, logger, &traceAPI{url: config.rpc})
	}

	walker := &walker{
		feeder: newSource(
			client,
			config,
			logger,
			&feederAPI{url: config.network.FeederURL, apiKey: config.apiKey},
		),
		preConfirmed: preConfirmed,
		dataset:      dataset,
		config:       config,
		concurrency:  config.concurrency,
		logger:       logger,
	}

	blocks, err := walker.walk(ctx)
	if err != nil {
		return err
	}

	logger.Info("capture complete", zap.Int("blocks", len(blocks)))
	return nil
}

func newClient(config *config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableCompression = true
	transport.MaxIdleConnsPerHost = config.concurrency
	return &http.Client{Transport: transport, Timeout: config.captureTimeout}
}

func newSource(
	client *http.Client,
	config *config,
	logger *log.ZapLogger,
	api api,
) *source {
	return &source{
		client:     client,
		retries:    config.captureRetries,
		retryDelay: config.captureRetryDelay,
		logger:     logger,
		api:        api,
	}
}

func (source *source) fetch(
	ctx context.Context,
	dataset dataset,
	resource resource,
) ([]byte, error) {
	body, err := dataset.read(resource.file)
	if err == nil {
		return body, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	body, err = source.download(ctx, dataset, resource)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", resource.file, err)
	}

	if err := dataset.write(resource.file, body); err != nil {
		return nil, err
	}

	return body, nil
}

func (source *source) download(
	ctx context.Context,
	dataset dataset,
	resource resource,
) ([]byte, error) {
	body, encoding, err := source.retrieve(ctx, resource)
	if err != nil {
		return nil, err
	}

	return source.api.decode(dataset, resource, body, encoding)
}

func (source *source) retrieve(
	ctx context.Context,
	resource resource,
) ([]byte, string, error) {
	body, encoding, err := source.retrieveOnce(ctx, resource)
	for attempt := 1; attempt <= source.retries && retryable(err); attempt++ {
		source.logger.Debug(
			"retrying capture request",
			zap.String("resource", resource.file),
			zap.Int("attempt", attempt),
			zap.Error(err),
		)
		if err := source.waitRetry(ctx); err != nil {
			return nil, "", err
		}

		body, encoding, err = source.retrieveOnce(ctx, resource)
	}

	if retryable(err) {
		return nil, "", fmt.Errorf("after %d attempts: %w", source.retries+1, err)
	}

	return body, encoding, err
}

func retryable(err error) bool {
	var status *statusError
	permanent := errors.As(err, &status) && status.code == http.StatusBadRequest
	return err != nil && !permanent
}

func (source *source) waitRetry(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(source.retryDelay):
		return nil
	}
}

func (source *source) retrieveOnce(
	ctx context.Context,
	resource resource,
) ([]byte, string, error) {
	request, err := source.api.newRequest(ctx, resource)
	if err != nil {
		return nil, "", err
	}

	request.Header.Set("Accept-Encoding", "gzip")
	response, err := source.client.Do(request)
	if err != nil {
		return nil, "", err
	}

	defer response.Body.Close()
	return readBody(response)
}

func readBody(response *http.Response) (body []byte, encoding string, err error) {
	if response.StatusCode != http.StatusOK {
		return nil, "", &statusError{code: response.StatusCode}
	}

	body, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}

	return body, response.Header.Get("Content-Encoding"), nil
}

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"time"

	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type source struct {
	feederURL  *url.URL
	client     *http.Client
	apiKey     string
	retries    int
	retryDelay time.Duration
	logger     *log.ZapLogger
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
		zap.String("flags", junoFlags(config.network, config.listen)),
	)
	logger.Info(
		"capturing",
		zap.Stringer("network", config.network),
		zap.Uint64("from", config.from),
		zap.Uint64("to", config.to),
	)
	walker := &walker{
		fetcher:     newSource(config, logger),
		dataset:     dataset,
		config:      config,
		concurrency: config.concurrency,
		logger:      logger,
	}
	blocks, err := walker.walk(ctx)
	if err != nil {
		return err
	}
	logger.Info("capture complete", zap.Int("blocks", len(blocks)))
	return nil
}

func newSource(config *config, logger *log.ZapLogger) *source {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableCompression = true
	transport.MaxIdleConnsPerHost = config.concurrency

	return &source{
		feederURL:  config.network.FeederURL,
		client:     &http.Client{Transport: transport, Timeout: config.captureTimeout},
		apiKey:     config.apiKey,
		retries:    config.captureRetries,
		retryDelay: config.captureRetryDelay,
		logger:     logger,
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

	body, err = source.download(ctx, resource.url(source.feederURL))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", resource.file, err)
	}
	if err := dataset.write(resource.file, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (source *source) download(ctx context.Context, requestURL *url.URL) ([]byte, error) {
	body, err := source.downloadOnce(ctx, requestURL)
	for attempt := 1; attempt <= source.retries && retryable(err); attempt++ {
		source.logger.Debug(
			"retrying capture request",
			zap.Stringer("url", requestURL),
			zap.Int("attempt", attempt),
			zap.Error(err),
		)
		if err := source.waitRetry(ctx); err != nil {
			return nil, err
		}
		body, err = source.downloadOnce(ctx, requestURL)
	}
	if retryable(err) {
		return nil, fmt.Errorf("after %d attempts: %w", source.retries+1, err)
	}
	return body, err
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

func (source *source) downloadOnce(ctx context.Context, requestURL *url.URL) ([]byte, error) {
	request, err := source.newRequest(ctx, requestURL)
	if err != nil {
		return nil, err
	}
	response, err := source.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return readBody(response)
}

func (source *source) newRequest(ctx context.Context, requestURL *url.URL) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), http.NoBody)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept-Encoding", "gzip")
	if source.apiKey != "" {
		request.Header.Set("X-Throttling-Bypass", source.apiKey)
	}
	return request, nil
}

func readBody(response *http.Response) ([]byte, error) {
	if response.StatusCode != http.StatusOK {
		return nil, &statusError{code: response.StatusCode}
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return ensureGzipped(body, response.Header.Get("Content-Encoding"))
}

func ensureGzipped(body []byte, contentEncoding string) ([]byte, error) {
	switch contentEncoding {
	case "gzip":
		return body, nil
	case "", "identity":
		return gzipBytes(body)
	default:
		return nil, fmt.Errorf("unsupported Content-Encoding %q", contentEncoding)
	}
}

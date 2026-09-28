package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type feederAPI struct {
	url    *url.URL
	apiKey string
}

func (feeder *feederAPI) newRequest(ctx context.Context, resource resource) (*http.Request, error) {
	requestURL := resource.url(feeder.url).String()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	if feeder.apiKey != "" {
		request.Header.Set("X-Throttling-Bypass", feeder.apiKey)
	}

	return request, nil
}

func (feeder *feederAPI) decode(
	_ dataset,
	_ resource,
	body []byte,
	encoding string,
) ([]byte, error) {
	switch encoding {
	case "gzip":
		return body, nil
	case "", "identity":
		return gzipBytes(body)
	default:
		return nil, fmt.Errorf("unsupported Content-Encoding %q", encoding)
	}
}

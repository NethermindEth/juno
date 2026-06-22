package feeder

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// Validatable is a generic constraint satisfied by a pointer type *T whose
// underlying value type T can validate itself. Implementers provide a
// Validate method with a pointer receiver that checks the receiver's fields
// and returns itself with no changes in case of success, or an error if validation fails.
type Validatable[T any] interface {
	*T
	Validate() (*T, error)
}

func doRequest[T any, V Validatable[T]](
	ctx context.Context,
	client *Client,
	queryURL string,
) (*T, error) {
	var result T
	body, err := client.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(&result); err != nil {
		return nil, err
	}
	return V(&result).Validate()
}

// buildQueryString builds the query url with encoded parameters
func (c *Client) buildQueryString(endpoint string, args map[string]string) string {
	base, err := url.Parse(c.url)
	if err != nil {
		panic("Malformed feeder base URL")
	}

	base.Path += endpoint

	params := url.Values{}
	for k, v := range args {
		params.Add(k, v)
	}
	base.RawQuery = params.Encode()

	return base.String()
}

// get performs a "GET" http request with the given URL and returns the response body
func (c *Client) get(ctx context.Context, queryURL string) (io.ReadCloser, error) {
	var res *http.Response
	var err error
	wait := time.Duration(0)
	for range c.maxRetries + 1 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
			var req *http.Request
			req, err = http.NewRequestWithContext(ctx, http.MethodGet, queryURL, http.NoBody)
			if err != nil {
				return nil, err
			}
			if c.userAgent != "" {
				req.Header.Set("User-Agent", c.userAgent)
			}
			if c.apiKey != "" {
				req.Header.Set("X-Throttling-Bypass", c.apiKey)
			}

			timeouts := c.timeouts.Load()
			c.client.Timeout = timeouts.GetCurrentTimeout()
			reqTimer := time.Now()
			res, err = c.client.Do(req)
			tooManyRequests, badRequest := false, false
			if err == nil {
				c.listener.OnResponse(req.URL.Path, res.StatusCode, time.Since(reqTimer))
				tooManyRequests = res.StatusCode == http.StatusTooManyRequests
				badRequest = res.StatusCode == http.StatusBadRequest
				if res.StatusCode == http.StatusOK {
					timeouts.DecreaseTimeout()
					return res.Body, nil
				} else {
					err = errors.New(res.Status)
				}

				res.Body.Close()
			}

			if !tooManyRequests && !badRequest {
				timeouts.IncreaseTimeout()
			}

			if wait < c.minWait {
				wait = c.minWait
			} else {
				wait = min(c.backoff(wait), c.maxWait)
			}

			currentTimeout := timeouts.GetCurrentTimeout()
			if currentTimeout >= mediumGrowThreshold {
				c.logger.Warn("Failed query to feeder, retrying...",
					zap.String("req", log.SanitizeString(req.URL.String())),
					zap.String("retryAfter", wait.String()),
					zap.Error(err),
					zap.String("newHTTPTimeout", currentTimeout.String()),
				)
				c.logger.Warn("Timeouts can be updated via HTTP PUT request",
					zap.String("timeout", currentTimeout.String()),
					zap.String("hint",
						`Set --http-update-port and --http-update-host flags and `+
							`make a PUT request to "/feeder/timeouts" with the specified timeouts`,
					),
				)
			} else {
				c.logger.Debug("Failed query to feeder, retrying...",
					zap.String("req", log.SanitizeString(req.URL.String())),
					zap.String("retryAfter", wait.String()),
					zap.Error(err),
					zap.String("newHTTPTimeout", currentTimeout.String()),
				)
			}
		}
	}
	return nil, err
}

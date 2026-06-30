package feeder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

var ErrInvalidFeederResponse = errors.New("invalid feeder response")

// Validatable is a generic constraint satisfied by a pointer type *T whose
// underlying value type T can validate itself. Implementers provide a
// Validate method with a pointer receiver that checks the receiver's fields
// and returns an error if validation fails.
type Validatable[T any] interface {
	*T
	Validate() error
}

func doRequest[T any, V Validatable[T]](
	ctx context.Context,
	client *Client,
	fullURL *url.URL,
) (T, error) {
	var result T
	body, err := client.get(ctx, fullURL)
	if err != nil {
		return result, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(&result); err != nil {
		return result, err
	}

	err = V(&result).Validate()
	if err != nil {
		return result, errors.Join(
			ErrInvalidFeederResponse,
			fmt.Errorf("querying %s: %w", fullURL, err),
		)
	}

	return result, nil
}

// buildQueryString builds the full URL with encoded parameters
func buildQueryString(baseURL *url.URL, endpoint string, args map[string]string) *url.URL {
	base := baseURL.JoinPath(endpoint)

	params := url.Values{}
	for k, v := range args {
		params.Add(k, v)
	}
	base.RawQuery = params.Encode()

	return base
}

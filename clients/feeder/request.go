package feeder

import (
	"context"
	"encoding/json"
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

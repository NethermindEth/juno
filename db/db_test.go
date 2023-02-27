package db_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

type errorer struct{}

func (e errorer) Close() error {
	return errors.New("closer error")
}

func TestCloseAndJoinOnError(t *testing.T) {
	var err error
	defer func() {
		db.CloseAndWrapOnError(errorer{}.Close, &err)
		assert.EqualError(t, errors.Unwrap(err), "closer error")
	}()

	db.CloseAndWrapOnError(errorer{}.Close, &err)
	assert.EqualError(t, err, "closer error")
}

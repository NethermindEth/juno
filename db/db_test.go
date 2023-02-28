package db_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

func closeAndError() error {
	return errors.New("close error")
}

func closeAndNoError() error {
	return nil
}

func TestCloseAndJoinOnError(t *testing.T) {
	t.Run("closeFn returns no error", func(t *testing.T) {
		t.Run("original error is nil", func(t *testing.T) {
			var err error
			db.CloseAndWrapOnError(closeAndNoError, &err)
		})

		t.Run("original error is non-nil", func(t *testing.T) {
			err := errors.New("some error")
			db.CloseAndWrapOnError(closeAndNoError, &err)
			assert.EqualError(t, err, "some error")
		})
	})
	t.Run("closeFn returns error", func(t *testing.T) {
		t.Run("original error is nil", func(t *testing.T) {
			var err error
			db.CloseAndWrapOnError(closeAndError, &err)
			assert.EqualError(t, err, "close error")
		})

		t.Run("original error is non-nil", func(t *testing.T) {
			err := errors.New("some error")
			db.CloseAndWrapOnError(closeAndError, &err)
			assert.EqualError(t, err, fmt.Sprintf(`failed to close because "%v" with existing err "%s"`, "close error",
				"some error"))
			assert.EqualError(t, errors.Unwrap(err), "some error")
		})
	})
}

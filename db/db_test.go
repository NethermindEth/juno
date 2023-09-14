package db_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

var errClose = errors.New("close error")

func closeAndError() error {
	return errClose
}

func closeAndNoError() error {
	return nil
}

func TestCloseAndJoinOnError(t *testing.T) {
	t.Run("closeFn returns no error", func(t *testing.T) {
		t.Run("original error is nil", func(t *testing.T) {
			assert.NoError(t, closeAndNoError())
		})

		t.Run("original error is non-nil", func(t *testing.T) {
			want := errors.New("some error")
			got := utils.RunAndWrapOnError(closeAndNoError, want)
			assert.EqualError(t, got, want.Error())
		})
	})
	t.Run("closeFn returns error", func(t *testing.T) {
		t.Run("original error is nil", func(t *testing.T) {
			assert.EqualError(t, closeAndError(), errClose.Error())
		})

		t.Run("original error is non-nil", func(t *testing.T) {
			want := errors.New("some error")
			wrapped := utils.RunAndWrapOnError(closeAndError, want)
			assert.EqualError(t, wrapped, fmt.Sprintf(`failed to run because "%v" with existing err %q`, errClose.Error(), want.Error()))
			assert.EqualError(t, errors.Unwrap(wrapped), want.Error())
		})
	})
}

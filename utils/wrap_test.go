package utils_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

var errRun = errors.New("run error")

func runAndError() error {
	return errRun
}

func runAndNoError() error {
	return nil
}

func TestRunAndWrapOnError(t *testing.T) {
	t.Run("runFn returns no error", func(t *testing.T) {
		t.Run("original error is nil", func(t *testing.T) {
			assert.NoError(t, runAndNoError())
		})

		t.Run("original error is non-nil", func(t *testing.T) {
			want := errors.New("some error")
			got := utils.RunAndWrapOnError(runAndNoError, want)
			assert.EqualError(t, got, want.Error())
		})
	})
	t.Run("runFn returns error", func(t *testing.T) {
		t.Run("original error is nil", func(t *testing.T) {
			assert.EqualError(t, runAndError(), errRun.Error())
		})

		t.Run("original error is non-nil", func(t *testing.T) {
			want := errors.New("some error")
			wrapped := utils.RunAndWrapOnError(runAndError, want)
			assert.EqualError(t, wrapped, fmt.Sprintf(`failed to run because "%v" with existing err %q`, errRun.Error(), want.Error()))
			assert.EqualError(t, errors.Unwrap(wrapped), want.Error())
		})
	})
}

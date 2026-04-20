package utils_test

import (
	"errors"
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
			old := utils.RunAndWrapOnError(runAndNoError, nil)
			replacement := errors.Join(nil, runAndNoError())

			assert.Equal(t, old == nil, replacement == nil, "nil behavior mismatch: old=%v replacement=%v", old, replacement)
		})

		t.Run("original error is non-nil", func(t *testing.T) {
			existingErr := errors.New("some error")
			old := utils.RunAndWrapOnError(runAndNoError, existingErr)
			replacement := errors.Join(existingErr, runAndNoError())

			assert.EqualError(t, old, existingErr.Error())
			assert.EqualError(t, replacement, existingErr.Error())
		})
	})
	t.Run("runFn returns error", func(t *testing.T) {
		t.Run("original error is nil", func(t *testing.T) {
			old := utils.RunAndWrapOnError(runAndError, nil)
			replacement := errors.Join(nil, runAndError())

			assert.EqualError(t, old, errRun.Error())
			assert.EqualError(t, replacement, errRun.Error())
		})

		t.Run("original error is non-nil", func(t *testing.T) {
			existingErr := errors.New("some error")
			old := utils.RunAndWrapOnError(runAndError, existingErr)
			replacement := errors.Join(existingErr, runAndError())

			// Both wrap existingErr
			assert.True(t, errors.Is(old, existingErr), "old does not wrap existingErr")
			assert.True(t, errors.Is(replacement, existingErr), "replacement does not wrap existingErr")
			// old uses %v for runErr (not unwrappable), errors.Join wraps both
			assert.False(t, errors.Is(old, errRun), "old wraps errRun (unexpected)")
			assert.True(t, errors.Is(replacement, errRun), "replacement does not wrap errRun")
		})
	})
}

package pipeline

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipeline(t *testing.T) {
	t.Run("nil channel", func(t *testing.T) {
		outValue := Stage(context.Background(), nil, strconv.Itoa)
		done := End(outValue, func(string) {
			// this function should not be called
			t.Fail()
		})

		_, open := <-done
		assert.False(t, open)
	})
}

// Todo: Add tests for fan in and bridge functions
// Consider adding a whole pipeline as a form of a test.

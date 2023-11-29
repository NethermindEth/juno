package utils

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func printStr(v string) {
	fmt.Println(v)
}

func TestPipeline(t *testing.T) {
	t.Run("nil channel", func(t *testing.T) {
		outValue := Pipeline(nil, strconv.Itoa)
		done := PipelineEnd(outValue, func(string) {
			// this function should not be called
			t.Fail()
		})

		_, open := <-done
		assert.False(t, open)
	})
}

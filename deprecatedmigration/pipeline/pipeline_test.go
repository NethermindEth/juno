package pipeline_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/deprecatedmigration/pipeline"
	"github.com/stretchr/testify/require"
)

var (
	concurrency = 16
	itemCount   = 10000
)

type state struct {
	count []int
	done  []bool
}

func (s *state) Run(index, input int, outputs chan<- uint64) error {
	s.count[index]++
	outputs <- testTransform(input)
	return nil
}

func (s *state) Done(index int, outputs chan<- uint64) error {
	s.done[index] = true
	return nil
}

func generateInputs() []int {
	inputs := make([]int, itemCount)
	for i := range itemCount {
		inputs[i] = rand.Int()
	}
	return inputs
}

func generateOutputs(inputs []int) []uint64 {
	outputs := make([]uint64, len(inputs))
	for i := range inputs {
		outputs[i] = testTransform(inputs[i])
	}
	return outputs
}

func testTransform(input int) uint64 {
	return uint64(input) * 2
}

func TestPipeline(t *testing.T) {
	inputs := make(chan int)
	s := state{
		count: make([]int, concurrency),
		done:  make([]bool, concurrency),
	}
	p := pipeline.New(inputs, concurrency, &s)

	inputData := generateInputs()
	go func() {
		for _, input := range inputData {
			inputs <- input
		}
		close(inputs)
		require.NoError(t, p.Wait())
	}()

	t.Run("Assert outputs", func(t *testing.T) {
		expectedOutputs := generateOutputs(inputData)

		actualOutputs := make([]uint64, 0, itemCount)
		for output := range p.Outputs() {
			actualOutputs = append(actualOutputs, output)
		}

		require.ElementsMatch(t, expectedOutputs, actualOutputs)
	})

	t.Run("Assert state", func(t *testing.T) {
		totalCount := 0
		for _, count := range s.count {
			totalCount += count
		}
		require.Equal(t, itemCount, totalCount)

		require.Equal(t, slices.Repeat([]bool{true}, concurrency), s.done)
	})
}

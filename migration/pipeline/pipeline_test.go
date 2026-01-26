package pipeline_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/migration/pipeline"
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
	p := pipeline.New(context.Background(), inputs, concurrency, &s)

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

type errorState struct {
	errorOnInput int
}

func (e *errorState) Run(index, input int, outputs chan<- uint64) error {
	if input == e.errorOnInput {
		return errors.New("test error")
	}
	outputs <- testTransform(input)
	return nil
}

func (e *errorState) Done(index int, outputs chan<- uint64) error {
	return nil
}

func TestPipeline_WorkerErrorCancelsContext(t *testing.T) {
	inputs := make(chan int)
	errorState := &errorState{errorOnInput: 5}
	p := pipeline.New(context.Background(), inputs, 2, errorState)

	// Consume output channel
	wg := sync.WaitGroup{}
	wg.Go(func() {
		for range p.Outputs() {
		}
	})

	timeout, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
outerloop:
	for i := range 100 {
		select {
		case <-p.Context().Done():
			break outerloop
		case <-timeout.Done():
			t.Fatal("timeout: generator did not stop after pipeline context was cancelled")
			break outerloop
		case inputs <- i:
		}
	}
	close(inputs)

	err := p.Wait()
	require.Error(t, err)
	require.Contains(t, err.Error(), "test error")
	wg.Wait()
}

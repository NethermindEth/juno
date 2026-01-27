package pipeline_test

import (
	"context"
	"errors"
	"iter"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/stretchr/testify/require"
)

const (
	concurrency = 16
	itemCount   = 50000
	failRange   = concurrency * 10
)

type state[I, O any] struct {
	transform func(I) (O, error)
	count     []int
	done      []bool
}

func newState[I, O any](transform func(I) (O, error)) state[I, O] {
	return state[I, O]{
		transform: transform,
		count:     make([]int, concurrency),
		done:      make([]bool, concurrency),
	}
}

func (s *state[I, O]) Run(index int, input I, outputs chan<- O) error {
	s.count[index]++
	o, err := s.transform(input)
	if err != nil {
		return err
	}
	outputs <- o
	return nil
}

func (s *state[I, O]) Done(index int, outputs chan<- O) error {
	s.done[index] = true
	return nil
}

func (s *state[I, O]) Count() int {
	totalCount := 0
	for _, count := range s.count {
		totalCount += count
	}
	return totalCount
}

func generateInputs() []int {
	inputs := make([]int, itemCount)
	for i := range itemCount {
		inputs[i] = rand.Int()
	}
	return inputs
}

func generateOutputs(t *testing.T, inputs []int) []uint64 {
	t.Helper()
	outputs := make([]uint64, len(inputs))
	for i := range inputs {
		var err error
		outputs[i], err = testTransform(inputs[i])
		require.NoError(t, err)
	}
	return outputs
}

func testTransform(input int) (uint64, error) {
	return uint64(input) * 2, nil
}

func TestPipeline(t *testing.T) {
	inputData := generateInputs()
	source := pipeline.Source(slices.Values(inputData))

	s := newState(testTransform)
	process := pipeline.New(source, concurrency, &s)
	outputs, wait := process.Run(t.Context())

	t.Run("Assert outputs", func(t *testing.T) {
		expectedOutputs := generateOutputs(t, inputData)

		actualOutputs := make([]uint64, 0, itemCount)
		for output := range outputs {
			actualOutputs = append(actualOutputs, output)
		}

		require.ElementsMatch(t, expectedOutputs, actualOutputs)
		require.NoError(t, wait())
	})

	t.Run("Assert state", func(t *testing.T) {
		require.Equal(t, itemCount, s.Count())
		require.Equal(t, slices.Repeat([]bool{true}, concurrency), s.done)
	})
}

func failAfterN(n int, err error) func(int) (int, error) {
	return func(input int) (int, error) {
		if input >= n && input < n+failRange {
			return 0, err
		}
		return input * 2, nil
	}
}

func source(itemCount int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range itemCount {
			if !yield(i) {
				return
			}
		}
	}
}

type failTestCase struct {
	name          string
	source        iter.Seq[int]
	parts         []int
	expectedError error
}

var errFailedExecution = errors.New("test error")

func testCase(name string, parts ...int) failTestCase {
	return failTestCase{
		name:          name,
		source:        source(itemCount),
		parts:         parts,
		expectedError: errFailedExecution,
	}
}

func assertFailureState(
	t *testing.T,
	ctx context.Context,
	testCase failTestCase,
) {
	t.Helper()
	t.Run(testCase.name, func(t *testing.T) {
		last := pipeline.Source(testCase.source)
		states := make([]state[int, int], len(testCase.parts))
		for i, part := range testCase.parts {
			states[i] = newState(failAfterN(part, errFailedExecution))
			last = pipeline.New(last, concurrency, &states[i])
		}

		t.Run("Assert error", func(t *testing.T) {
			outputs, wait := last.Run(ctx)
			for range outputs {
			}
			err := wait()
			if testCase.expectedError != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, testCase.expectedError)
			} else {
				require.NoError(t, err)
			}
		})

		t.Run("Assert state", func(t *testing.T) {
			for _, state := range states {
				require.Equal(t, state.done, slices.Repeat([]bool{true}, concurrency))
				if testCase.expectedError != nil {
					require.Less(t, state.Count(), itemCount)
				} else {
					require.Equal(t, state.Count(), itemCount)
				}
			}
		})
	})
}

func runTestPipelineError(t *testing.T, testCases ...failTestCase) {
	t.Helper()
	for _, testCase := range testCases {
		assertFailureState(t, t.Context(), testCase)
	}
}

func TestPipelineError(t *testing.T) {
	const (
		early   = 10
		late    = 1000
		success = itemCount + 1
	)
	t.Run("2 parts", func(t *testing.T) {
		runTestPipelineError(
			t,
			testCase("first fails, second succeeds", early, success),
			testCase("first succeeds, second fails", success, early),
			testCase("first fails before second", early, late),
			testCase("first fails after second", late, early),
		)
	})
	t.Run("3 parts", func(t *testing.T) {
		t.Run("1 failure", func(t *testing.T) {
			runTestPipelineError(
				t,
				testCase("first fails, second and third succeed", early, success, success),
				testCase("first succeeds, second fails, third succeeds", success, early, success),
				testCase("first and second succeed, third fails", success, success, early),
			)
		})
		t.Run("2 failures", func(t *testing.T) {
			runTestPipelineError(
				t,
				testCase("first fails before second, third succeeds", early, late, success),
				testCase("first fails after second, third succeeds", late, early, success),
				testCase("first fails before third, second succeeds", early, success, late),
				testCase("first fails after third, second succeeds", late, success, early),
				testCase("first succeeds, second fails before third", success, early, late),
				testCase("first succeeds, second fails after third", success, late, early),
				testCase("first and second fail together", early, early, late),
				testCase("first and third fail together", early, late, early),
				testCase("second and third fail together", late, early, early),
			)
		})
		t.Run("3 failures", func(t *testing.T) {
			runTestPipelineError(
				t,
				testCase("all fail", late, late, late),
			)
		})
	})
	t.Run("Multiple parts", func(t *testing.T) {
		const parts = 100
		oneFailure := slices.Repeat([]int{success}, parts)
		oneFailure[rand.IntN(parts)] = late
		runTestPipelineError(
			t,
			testCase("all fail", slices.Repeat([]int{late}, parts)...),
			testCase("one fails", oneFailure...),
		)
	})
	t.Run("Source cancel", func(t *testing.T) {
		const parts = 100
		ctx, cancel := context.WithCancel(t.Context())
		source := func(yield func(int) bool) {
			for i := range itemCount {
				if !yield(i) {
					return
				}
				if i == early {
					cancel()
				}
			}
		}
		assertFailureState(
			t,
			ctx,
			failTestCase{
				name:          "Source cancel",
				source:        source,
				parts:         slices.Repeat([]int{success}, parts),
				expectedError: context.Canceled,
			},
		)
	})
}

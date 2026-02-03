package pipeline

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipeline(t *testing.T) {
	t.Run("nil channel", func(t *testing.T) {
		t.Run("Stage", func(t *testing.T) {
			_, open := <-Stage(t.Context(), nil, strconv.Itoa)
			assert.False(t, open)
		})
		t.Run("FanIn", func(t *testing.T) {
			_, open := <-FanIn[int](t.Context(), nil, nil, nil)
			assert.False(t, open)
		})
		t.Run("Bridge doesn't call out", func(t *testing.T) {
			out := make(chan int)
			Bridge[int](t.Context(), out, nil) // non-blocking
		})
	})

	t.Run("ctx is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		t.Run("Stage", func(t *testing.T) {
			_, open := <-Stage(ctx, nil, strconv.Itoa)
			assert.False(t, open)
		})
		t.Run("FanIn", func(t *testing.T) {
			_, open := <-FanIn[int](ctx, nil, nil, nil)
			assert.False(t, open)
		})
		t.Run("Bridge", func(t *testing.T) {
			out := make(chan int)
			chOfInts := make(chan (<-chan int))
			Bridge(ctx, out, chOfInts) // non-blocking
		})
	})

	t.Run("sort numbers in pipeline", func(t *testing.T) {
		// map is being used because ranging over a map is random
		nums := map[string]struct{}{
			"0": {}, "1": {}, "2": {},
			"3": {}, "4": {}, "5": {},
			"6": {}, "7": {}, "8": {},
			"9": {},
		}

		chs := make([]<-chan string, 0, len(nums))

		for n := range nums {
			strCh := make(chan string)
			go func() {
				defer close(strCh)
				strCh <- n
			}()
			chs = append(chs, strCh)
		}

		stringToInt := func(s string) int {
			i, err := strconv.Atoi(s)
			require.NoError(t, err)
			return i
		}

		sleepOnInt := func(n int) <-chan int {
			ch := make(chan int)

			go func() {
				defer close(ch)
				time.Sleep(time.Duration(rand.Intn(5)+1) * time.Microsecond)
				ch <- n
			}()

			return ch
		}

		ctx := t.Context()
		chOfInts := make(chan (<-chan int))
		nextVal := -1

		go func() {
			defer close(chOfInts)

			intM := make(map[int]struct{})

			for i := range Stage(ctx, FanIn(ctx, chs...), stringToInt) {
				intM[i] = struct{}{}

				if _, ok := intM[nextVal+1]; ok {
					chOfInts <- sleepOnInt(nextVal + 1)
					delete(intM, nextVal+1)
					nextVal++
				}
			}

			for len(intM) != 0 {
				if _, ok := intM[nextVal+1]; !ok {
					continue
				}

				chOfInts <- sleepOnInt(nextVal + 1)
				delete(intM, nextVal+1)
				nextVal++
			}
		}()

		out := make(chan int)
		go Bridge[int](ctx, out, chOfInts) // Blocking writes into `out`

		i := 0
		for n := range out {
			assert.Equal(t, i, n)
			i++

			// All values have been ordered, so `out` has completed its task
			if i == len(nums) {
				break
			}
		}
	})
}

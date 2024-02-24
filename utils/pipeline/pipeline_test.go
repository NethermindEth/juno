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
			_, open := <-Stage(context.Background(), nil, strconv.Itoa)
			assert.False(t, open)
		})
		t.Run("FanIn", func(t *testing.T) {
			_, open := <-FanIn[int](context.Background(), nil, nil, nil)
			assert.False(t, open)
		})
		t.Run("Bridge", func(t *testing.T) {
			_, open := <-Bridge[int](context.Background(), nil)
			assert.False(t, open)
		})
	})

	t.Run("ctx is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
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
			_, open := <-Bridge[int](ctx, nil)
			assert.False(t, open)
		})
	})

	t.Run("sort numbers in pipeline", func(t *testing.T) {
		// map is being used because ranging over a map is random
		nums := map[string]struct{}{"0": {}, "1": {}, "2": {}, "3": {}, "4": {}, "5": {}, "6": {}, "7": {}, "8": {}, "9": {}}

		var chs []<-chan string

		for n := range nums {
			n := n
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

		ctx := context.Background()
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

		i := 0
		for n := range Bridge(ctx, chOfInts) {
			assert.Equal(t, i, n)
			i++
		}
	})
}

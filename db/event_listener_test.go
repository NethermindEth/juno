package db_test

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/require"
)

type op string

const (
	delay       = 100 * time.Millisecond
	opRead   op = "OnIO Read"
	opWrite  op = "OnIO Write"
	opCommit op = "OnCommit"
)

type eventListenerTestCase struct {
	op op
	fn func(db.EventListener)
}

func TestEventListener(t *testing.T) {
	testCases := []eventListenerTestCase{
		{
			op: opRead,
			fn: func(listener db.EventListener) {
				defer listener.OnIO(false, time.Now())
				time.Sleep(delay)
			},
		},
		{
			op: opWrite,
			fn: func(listener db.EventListener) {
				defer listener.OnIO(true, time.Now())
				time.Sleep(delay)
			},
		},
		{
			op: opCommit,
			fn: func(listener db.EventListener) {
				defer listener.OnCommit(time.Now())
				time.Sleep(delay)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.op), func(t *testing.T) {
			var actualOp op
			var actualDuration time.Duration

			listener := db.SelectiveListener{
				OnIOCb: func(write bool, duration time.Duration) {
					if write {
						actualOp = opWrite
					} else {
						actualOp = opRead
					}
					actualDuration = duration
				},
				OnCommitCb: func(duration time.Duration) {
					actualOp = opCommit
					actualDuration = duration
				},
			}

			testCase.fn(&listener)
			require.Equal(t, testCase.op, actualOp)
			require.GreaterOrEqual(t, actualDuration, delay)
		})
	}
}

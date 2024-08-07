package consensus

import (
	"fmt"
	"time"
)

type RoundType = int64
type HeightType = uint64
type IdType = uint64

type Gossiper interface {
	SubmitMessageForBroadcast(msg interface{})   // adds a msg to be broadcast to the queue
	GetSubmittedMessage() interface{}            // takes a msg to be broadcast off the queue
	ReceiveMessageFromBroadcast(msg interface{}) // adds a msg received onto the queue
	GetReceivedMessage() interface{}             // takes a msg received off the queue
	ClearAll()
	ClearSubmit()
	ClearReceive()
}

type Proposer interface {
	Proposer(height HeightType, round RoundType) interface{}
	IsProposer(height HeightType, round RoundType) uint8      // 0 no, 1 yes, 2 unknown
	StrictIsProposer(height HeightType, round RoundType) bool // yes or no
	Elect(height HeightType, round RoundType) bool            // true if node is selected, false otherwise
	Propose(height HeightType, round RoundType) Proposable
}

type Decider interface {
	SubmitDecision(decision Proposable, height HeightType) bool
	GetDecision(height HeightType) interface{}
}

// Proposable todo: use pointers for memory efficiency
type Proposable interface {
	Id() IdType // should be a hash
	Value() Proposable
	IsId() bool
	IsValue() bool
	IsValid() bool
	Equals(other interface{}) bool
	EqualsTo(other Proposable) bool
}

func SetTimeOut(f func(), t time.Duration) {
	go func() {
		time.Sleep(t)
		f()
	}()
}

func CheckTimeOut(title string, expected, t time.Duration) {
	SetTimeOut(func() {
		panic(fmt.Sprintf("%s time out took too long, longer than %s", title, expected.String()))
	}, t)
}

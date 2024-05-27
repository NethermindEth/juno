package consensus

import "time"

type Gossiper interface {
	SubmitMessageForBroadcast(msg interface{})   // adds a msg to be broadcast to the queue
	SubmitMessage(msg interface{})               // takes a msg to be broadcast off the queue
	ReceiveMessageFromBroadcast(msg interface{}) // adds a msg received onto the queue
	ReceiveMessage() interface{}                 // takes a msg received off the queue
	ClearAll()
	ClearSubmit()
	ClearReceive()
}

type Proposer interface {
	Proposer(height, round uint64) interface{}
	IsProposer(height, round uint64) uint8      // 0 no, 1 yes, 2 unknown
	StrictIsProposer(height, round uint64) bool // yes or no
	Elect(height, round uint64) bool            // true if node is selected, false otherwise
	Propose(height, round uint64) Proposable
}

type Decider interface {
	SubmitDecision(decision *Proposable, height uint64) bool
	GetDecision(height uint64) interface{}
}

type Proposable interface {
	Id() Proposable
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

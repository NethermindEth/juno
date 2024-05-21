package consensus

type Gossiper interface {
	SubmitMessageForBroadcast(msg interface{})   // adds a msg to be broadcast to the queue
	SubmitMessage(msg interface{})               // takes a msg to be broadcast off the queue
	ReceiveMessage() interface{}                 // takes a msg received off the queue
	ReceiveMessageFromBroadcast(msg interface{}) // adds a msg received onto the queue
	ClearAll()
	ClearSubmit()
	ClearReceive()
}

type Proposer interface {
	Proposer() interface{}
	IsProposer() uint8      // 0 no, 1 yes 2 unknown
	StrictIsProposer() bool // yes or no
}

type Decider interface {
	SubmitDecision(decision interface{})
	GetDecision(params map[string]interface{}) interface{}
}

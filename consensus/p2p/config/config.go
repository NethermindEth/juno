package config

import "time"

type TopicBufferSizes struct {
	// Inbound
	Subscription int
	Output       int
	// Outbound
	ProtoBroadcaster int
}

type BufferSizes struct {
	VoteSubscription          int
	ProposalDemux             int
	ProposalCommitNotifier    int
	ProposalSingleStreamInput int
	ProposalOutputs           int
	PrevoteOutput             int
	PrecommitOutput           int
	ProposalProtoBroadcaster  int
	VoteProtoBroadcaster      int
	PubSubQueueSize           int
	RetryInterval             time.Duration
}

var DefaultBufferSizes = BufferSizes{
	VoteSubscription:          1024,
	ProposalDemux:             1024,
	ProposalCommitNotifier:    1024,
	ProposalSingleStreamInput: 1024,
	ProposalOutputs:           1024,
	PrevoteOutput:             1024,
	PrecommitOutput:           1024,
	ProposalProtoBroadcaster:  1024,
	VoteProtoBroadcaster:      1024,
	PubSubQueueSize:           1024,
	RetryInterval:             1 * time.Second,
}

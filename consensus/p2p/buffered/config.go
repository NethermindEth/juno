package buffered

type BufferSizeConfig struct {
	ProposalSubscription      int
	VoteSubscription          int
	ProposalDemux             int
	ProposalCommitNotifier    int
	ProposalSingleStreamInput int
	ProposalOutputs           int
	PrevoteOutput             int
	PrecommitOutput           int
}

var DefaultBufferSizeConfig = BufferSizeConfig{
	ProposalSubscription:      1024,
	VoteSubscription:          1024,
	ProposalDemux:             1024,
	ProposalCommitNotifier:    32,
	ProposalSingleStreamInput: 32,
	ProposalOutputs:           32,
	PrevoteOutput:             1024,
	PrecommitOutput:           1024,
}

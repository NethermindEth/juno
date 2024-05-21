package consensus

type ProposerElection struct{}

func NewProposerElection() *ProposerElection {
	panic("implement me")
}

func (l *ProposerElection) Proposer() uint64 {
	panic("implement me")
}

// IsProposer this node is the proposer
func (l *ProposerElection) IsProposer(params map[string]interface{}) uint8 {
	// yes, no, unknown
	panic("implement me")
}

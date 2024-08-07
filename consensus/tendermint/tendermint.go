package tendermint

type TendermintConsensus struct {
}

func NewTendermintConsensus() *TendermintConsensus {

	// proposer elector
	// gossip message queues
	// decision bag
	panic("implement me")
}

func (consensus *TendermintConsensus) Init(params map[string]interface{}) error {
	panic("implement me")
}

func (consensus *TendermintConsensus) Run(params map[string]interface{}) error {
	panic("implement me")
}

// LargeIntValue todo: Experimental Large Int
type LargeIntValue struct {
	value int64
}

// LargeUIntValue todo: Experimental Unsigned Large Int
type LargeUIntValue struct {
	value uint64
}

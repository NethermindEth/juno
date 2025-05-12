package types

type (
	Step        uint8
	Height      uint
	Round       int
	VotingPower uint
)

const (
	StepPropose Step = iota
	StepPrevote
	StepPrecommit
)

func (s Step) String() string {
	switch s {
	case StepPropose:
		return "propose"
	case StepPrevote:
		return "prevote"
	case StepPrecommit:
		return "precommit"
	default:
		return "unknown"
	}
}

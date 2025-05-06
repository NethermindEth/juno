package tendermint

type IsWALMsg interface {
	msgType() MessageType
}

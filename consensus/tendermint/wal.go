package tendermint

type IsWALMsg interface {
	msgType() MessageType
	height() Height
}

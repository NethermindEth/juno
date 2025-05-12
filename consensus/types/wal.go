package types

type IsWALMsg interface {
	MsgType() MessageType
	GetHeight() Height
}

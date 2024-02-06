package blockchain

type EventListener interface {
	OnRead(method string)
}

type SelectiveListener struct {
	OnReadCb func(method string)
}

func (l *SelectiveListener) OnRead(method string) {
	if l.OnReadCb != nil {
		l.OnReadCb(method)
	}
}

package db

type EventListener interface {
	OnIO(write bool)
}

type SelectiveListener struct {
	OnIOCb func(write bool)
}

func (l *SelectiveListener) OnIO(write bool) {
	if l.OnIOCb != nil {
		l.OnIOCb(write)
	}
}

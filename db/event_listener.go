package db

import "github.com/cockroachdb/pebble"

type PebbleMetrics struct {
	Src *pebble.Metrics
}

type PebbleListener interface {
	OnPebbleMetrics(*PebbleMetrics)
}

type EventListener interface {
	OnIO(write bool)
	PebbleListener
}

type SelectiveListener struct {
	OnIOCb func(write bool)

	OnPebbleMetricsCb func(*PebbleMetrics)
}

func (l *SelectiveListener) OnIO(write bool) {
	if l.OnIOCb != nil {
		l.OnIOCb(write)
	}
}

func (l *SelectiveListener) OnPebbleMetrics(m *PebbleMetrics) {
	if l.OnPebbleMetricsCb != nil {
		l.OnPebbleMetricsCb(m)
	}
}

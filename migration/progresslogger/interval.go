package progresslogger

import (
	"context"
	"time"
)

// CallEveryInterval starts a goroutine that calls the provided function at regular intervals
func CallEveryInterval(ctx context.Context, interval time.Duration, fn func()) context.CancelFunc {
	//nolint:gosec // G118: cancel is returned to caller to consume
	loggerCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-loggerCtx.Done():
				return
			case <-ticker.C:
				fn()
			}
		}
	}()
	return cancel
}

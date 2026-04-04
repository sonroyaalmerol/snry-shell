package runner

import (
	"context"
	"time"
)

func PollLoop(ctx context.Context, interval time.Duration, poll func()) error {
	poll()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			poll()
		}
	}
}

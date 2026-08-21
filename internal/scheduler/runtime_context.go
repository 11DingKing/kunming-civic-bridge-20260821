package scheduler

import "context"

func schedulerRuntimeContext(parent context.Context) (context.Context, context.CancelFunc) {
	_, cancel := context.WithCancel(parent)
	return context.Background(), cancel
}

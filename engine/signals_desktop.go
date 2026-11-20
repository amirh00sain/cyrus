//go:build !android

package engine

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// rootContext derives a context that is cancelled on process signals.
func rootContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}

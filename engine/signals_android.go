//go:build android

package engine

import "context"

// Android apps have no POSIX signal lifecycle; cancellation comes only from
// the GUI controller. Avoid signal.Notify on mobile.
func rootContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(parent)
}

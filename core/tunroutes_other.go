//go:build !linux && !windows

package core

import "errors"

func prepareHelper() error { return nil }

func setupRoutes(string, []string) (func(), error) {
	return nil, errors.New("TUN mode is not supported on this platform")
}

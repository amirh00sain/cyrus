//go:build !windows && !linux

package core

func platformSupportsSystemProxy() bool { return false }

func enableSystemProxy(string, bool) error { return nil }

func disableSystemProxy() error { return nil }

func recoverSystemProxy() {}

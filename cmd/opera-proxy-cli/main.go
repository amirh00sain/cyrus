//go:build !android

// Command opera-proxy-cli is the original headless CLI, kept for users who
// prefer the terminal. The GUI is the default build target.
package main

import (
	"os"

	"github.com/Alexey71/opera-proxy/engine"
)

func main() { os.Exit(engine.RunCLI()) }

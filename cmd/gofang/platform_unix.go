//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func enableANSI() {
	// Unix terminals support ANSI natively, nothing to do.
}

func registerSignals(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
}

//go:build !windows
// +build !windows

package main

import (
	"errors"
	"out_heap_alarm_go/config"
)

// handleServiceCommand returns false on non-Windows platforms.
func handleServiceCommand(args []string) bool {
	return false
}

// isServiceRun is only meaningful on Windows.
func isServiceRun() bool {
	return false
}

// runService is only available on Windows.
func runService(cfg *config.Config, logFile string) error {
	return errors.New("service mode is only supported on Windows")
}

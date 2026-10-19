//go:build !windows

package main

// pauseIfOwnWindow does nothing: on macOS a program opened from Finder runs
// in a Terminal window that stays open, and Linux users start it themselves
func pauseIfOwnWindow() {}

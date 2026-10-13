//go:build !windows

package main

func isMaximized(title string) bool { return false }

func maximizeWhenReady(title string) {}

func screenSize() (w, h int32) { return 0, 0 }

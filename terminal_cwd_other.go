//go:build !darwin && !linux

package main

import "fmt"

type cwdBridge struct{}

func newCWDBridge(path string) (*cwdBridge, error) {
	return nil, fmt.Errorf("terminal directory synchronization requires zsh on macOS or Linux")
}
func (b *cwdBridge) env() []string                             { return nil }
func (b *cwdBridge) setDir(path string, revision uint64) error { return nil }
func (b *cwdBridge) close()                                    {}

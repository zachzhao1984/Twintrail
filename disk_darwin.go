package main

import (
	"fmt"
	"syscall"
)

func diskSpace(path string) string {
	var s syscall.Statfs_t
	if syscall.Statfs(path, &s) != nil || s.Blocks == 0 {
		return ""
	}
	free := float64(s.Bavail) * float64(s.Bsize) / (1 << 30)
	return fmt.Sprintf("%.2f GB (%.0f%%) available", free, float64(s.Bavail)/float64(s.Blocks)*100)
}

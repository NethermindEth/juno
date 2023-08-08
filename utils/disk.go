//go:build !windows
// +build !windows

package utils

import (
	"fmt"
	"syscall"
)

const GB = 1024 * 1024 * 1024

func AvailableDiskSpace(path string) (float64, error) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return 0, fmt.Errorf("couldn't get the available free disk space, make sure you have enough space for the sync")
	}
	freeSpaceGB := float64(fs.Bavail*uint64(fs.Bsize)) / float64(GB)

	return freeSpaceGB, nil
}

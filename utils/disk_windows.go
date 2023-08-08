//go:build windows
// +build windows

package utils

import (
	"syscall"
	"unsafe"
)

const GB = 1024*1024*1024

func AvailableDiskSpace(volumePath string) (float64, error) {
	var availBytes int64

	h := syscall.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	c.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(volumePath))), 0, 0, uintptr(unsafe.Pointer(&availBytes)))
	freeSpaceGB := float64(uint64(availBytes))/float64(GB)

	return freeSpaceGB, nil
}

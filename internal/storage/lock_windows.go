//go:build windows

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

const (
	lockfileExclusiveLock   = 0x00000002
	lockfileFailImmediately = 0x00000001
)

func acquireDirLockFile(dataDir string) (*os.File, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dataDir, ".beacon.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := lockFile(syscall.Handle(f.Fd())); err != nil {
		f.Close()
		return nil, fmt.Errorf("data directory locked (is the Beacon server running?)")
	}
	return f, nil
}

func releaseDirLockFile(f *os.File) {
	if f == nil {
		return
	}
	_ = unlockFile(syscall.Handle(f.Fd()))
	_ = f.Close()
}

func lockFile(h syscall.Handle) error {
	var ol syscall.Overlapped
	r, _, err := procLockFileEx.Call(
		uintptr(h),
		uintptr(lockfileExclusiveLock|lockfileFailImmediately),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r == 0 {
		if err == syscall.Errno(0) {
			return syscall.Errno(33) // ERROR_LOCK_VIOLATION
		}
		return err
	}
	return nil
}

func unlockFile(h syscall.Handle) error {
	var ol syscall.Overlapped
	r, _, err := procUnlockFileEx.Call(
		uintptr(h),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r == 0 {
		return err
	}
	return nil
}

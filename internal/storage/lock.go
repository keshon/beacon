package storage

import "os"

// DirLock is an exclusive process-wide lock on the Beacon data directory.
type DirLock struct {
	file *os.File
}

// AcquireDirLock prevents concurrent server/CLI access to dataDir.
func AcquireDirLock(dataDir string) (*DirLock, error) {
	f, err := acquireDirLockFile(dataDir)
	if err != nil {
		return nil, err
	}
	return &DirLock{file: f}, nil
}

// Release unlocks and closes the lock file.
func (l *DirLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	releaseDirLockFile(l.file)
	l.file = nil
}

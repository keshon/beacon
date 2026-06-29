package cli

import "os"

// AcquireServerLock takes an exclusive lock on the data directory for the server process.
func AcquireServerLock(dataDir string) (*os.File, error) {
	return acquireDataDirLock(dataDir)
}

// ReleaseDataDirLock releases a data-directory flock.
func ReleaseDataDirLock(f *os.File) {
	releaseDataDirLock(f)
}

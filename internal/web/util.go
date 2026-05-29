package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/keshon/buildinfo"
)

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func getBuildVersion() string {
	return buildinfo.Get().BuildTime + " " + buildinfo.Get().GoVersion + " (" + buildinfo.Get().Commit + ")"
}

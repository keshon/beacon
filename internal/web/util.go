package web

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/keshon/buildinfo"
)

func randomID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getBuildVersion() string {
	return buildinfo.Get().BuildTime + " " + buildinfo.Get().GoVersion + " (" + buildinfo.Get().Commit + ")"
}

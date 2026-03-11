package web

import (
	"crypto/rand"
	"encoding/hex"
)

func randomID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

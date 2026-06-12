package stringutil

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateId() string {
	b := make([]byte, 10)
	rand.Read(b)
	return hex.EncodeToString(b)
}

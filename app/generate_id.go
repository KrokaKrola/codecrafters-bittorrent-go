package main

import (
	"crypto/rand"
	"encoding/hex"
)

func generateId() string {
	b := make([]byte, 10)
	rand.Read(b)
	return hex.EncodeToString(b)
}

package stringutil

import (
	"crypto/sha1"
	"encoding/hex"
)

func CreateSha1HashFromString(input string, withEncoding bool) (string, error) {
	return CreateSha1Hash([]byte(input), withEncoding)
}

func CreateSha1Hash(input []byte, withEncoding bool) (string, error) {
	hash := sha1.New()

	if _, err := hash.Write(input); err != nil {
		return "", err
	}

	hashSum := hash.Sum(nil)

	if withEncoding {
		return hex.EncodeToString(hashSum), nil
	}

	return string(hashSum), nil
}

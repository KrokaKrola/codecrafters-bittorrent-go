package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
)

func encode(input any) (string, error) {
	switch value := input.(type) {
	case dictionary:
		var result strings.Builder

		for _, value := range value {
			key, err := encode(value.key)
			if err != nil {
				return "", err
			}

			value, err := encode(value.value)
			if err != nil {
				return "", err
			}

			fmt.Fprintf(&result, "%s%s", key, value)
		}

		return fmt.Sprintf("d%se", result.String()), nil
	case map[string]any:
		var result strings.Builder

		for key, value := range value {
			key, err := encode(key)
			if err != nil {
				return "", err
			}

			value, err := encode(value)
			if err != nil {
				return "", err
			}

			fmt.Fprintf(&result, "%s%s", key, value)
		}

		return fmt.Sprintf("d%se", result.String()), nil
	case string:
		return fmt.Sprintf("%d:%s", len(value), value), nil
	case int:
		return fmt.Sprintf("i%de", value), nil
	case []any:
		var result strings.Builder

		for _, el := range value {
			r, err := encode(el)
			if err != nil {
				return "", err
			}

			result.WriteString(r)
		}

		return fmt.Sprintf("l%se", result.String()), nil
	default:
		return "", fmt.Errorf("unknown datatype: %v", value)
	}
}

func createSha1HashFromString(input string, withEncoding bool) (string, error) {
	return createSha1Hash([]byte(input), withEncoding)
}

func createSha1Hash(input []byte, withEncoding bool) (string, error) {
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

package bencode

import (
	"fmt"
	"strings"
)

func Encode(input any) (string, error) {
	switch value := input.(type) {
	case Dictionary:
		var result strings.Builder

		for _, value := range value {
			key, err := Encode(value.Key)
			if err != nil {
				return "", err
			}

			value, err := Encode(value.Value)
			if err != nil {
				return "", err
			}

			fmt.Fprintf(&result, "%s%s", key, value)
		}

		return fmt.Sprintf("d%se", result.String()), nil
	case map[string]any:
		var result strings.Builder

		for key, value := range value {
			key, err := Encode(key)
			if err != nil {
				return "", err
			}

			value, err := Encode(value)
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
			r, err := Encode(el)
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

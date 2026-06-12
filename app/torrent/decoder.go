package torrent

import (
	"fmt"
	"strconv"
)

type dictionaryValue struct {
	key   string
	value any
}

type Dictionary []dictionaryValue

func (d Dictionary) ToMap() map[string]any {
	result := make(map[string]any)

	for _, el := range d {
		if innerDict, ok := el.value.(Dictionary); ok {
			el.value = innerDict.ToMap()
		}

		result[el.key] = el.value
	}

	return result
}

func findElementInDictionary[T any](d Dictionary, key string) (T, bool) {
	var res T

	for _, el := range d {
		if el.key == key {
			if value, ok := el.value.(T); ok {
				res = value
				return res, true
			} else {
				return res, false
			}
		}
	}

	return res, false
}

const (
	dictionaryBencodeIdentifier = byte('d')
	integerBencodeIdentifier    = byte('i')
	listBencodeIdentifier       = byte('l')
	endOfIdentifier             = byte('e')
	columnIdentifier            = byte(':')
)

type Decoder struct {
	Data []byte
	pos  int
	Err  error
}

func (d *Decoder) Decode() any {
	if d.Err != nil {
		return nil
	}

	if d.pos >= len(d.Data) {
		d.Err = fmt.Errorf("invalid input format: %s", d.Data)
		return nil
	}

	inputType := d.peekByte()

	switch inputType {
	case dictionaryBencodeIdentifier:
		d.readByte()
		result := Dictionary{}

		for {
			if d.Err != nil {
				return nil
			}

			if d.pos >= len(d.Data) {
				d.Err = fmt.Errorf("invalid dictionary format: %s", d.Data)
				return nil
			}

			if d.peekByte() == endOfIdentifier {
				d.readByte()
				break
			}

			key, ok := d.Decode().(string)
			if !ok {
				d.Err = fmt.Errorf("invalid dictionary format, each key must be string type: %s", d.Data)
				return nil
			}

			value := d.Decode()
			result = append(result, struct {
				key   string
				value any
			}{
				key,
				value,
			})
		}

		return result
	case integerBencodeIdentifier:
		d.readByte()
		startPos := d.pos
		var endOfNumber int

		for i := d.pos; i < len(d.Data); i++ {
			if d.readByte() == endOfIdentifier {
				endOfNumber = i
				break
			}
		}

		if endOfNumber == 0 {
			d.Err = fmt.Errorf("invalid number format: %s", d.Data)
			return nil
		}

		number, err := strconv.Atoi(string(d.Data[startPos:endOfNumber]))
		if err != nil {
			d.Err = err
			return nil
		}

		return number
	case listBencodeIdentifier:
		d.readByte()
		result := []any{}

		for {
			if d.Err != nil {
				return nil
			}

			if d.pos >= len(d.Data) {
				d.Err = fmt.Errorf("invalid list format: %s", d.Data)
				return nil
			}

			if d.peekByte() == endOfIdentifier {
				d.readByte()
				break
			}

			val := d.Decode()
			result = append(result, val)
		}

		return result
	default:
		initialPos := d.pos
		var firstColonIndex int

		for i := d.pos; i < len(d.Data); i++ {
			if d.readByte() == columnIdentifier {
				firstColonIndex = i
				break
			}
		}

		lengthStr := d.Data[initialPos:firstColonIndex]

		length, err := strconv.Atoi(string(lengthStr))
		if err != nil {
			d.Err = err
			return nil
		}

		if d.pos+length > len(d.Data) {
			d.Err = fmt.Errorf("invalid string length: %d", length)
			return nil
		}

		d.pos += length
		return string(d.Data[firstColonIndex+1 : d.pos])
	}
}

func (d *Decoder) readByte() byte {
	b := d.Data[d.pos]
	d.pos++
	return b
}

func (d *Decoder) peekByte() byte {
	b := d.Data[d.pos]
	return b
}

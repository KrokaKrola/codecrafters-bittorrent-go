package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

type dictionaryValue struct {
	key   string
	value any
}

type dictionary []dictionaryValue

func (d dictionary) toMap() map[string]any {
	result := make(map[string]any)

	for _, el := range d {
		if innerDict, ok := el.value.(dictionary); ok {
			el.value = innerDict.toMap()
		}

		result[el.key] = el.value
	}

	return result
}

var (
	dictionaryBencodeIdentifier = byte('d')
	integerBencodeIdentifier    = byte('i')
	listBencodeIdentifier       = byte('l')
	endOfIdentifier             = byte('e')
	columnIdentifier            = byte(':')
)

type decoder struct {
	data []byte
	pos  int
	err  error
}

func (d *decoder) decode() any {
	if d.err != nil {
		return nil
	}

	if d.pos >= len(d.data) {
		d.err = fmt.Errorf("invalid input format: %s", d.data)
		return nil
	}

	inputType := d.peekByte()

	switch inputType {
	case dictionaryBencodeIdentifier:
		d.readByte()
		result := dictionary{}

		for {
			if d.err != nil {
				return nil
			}

			if d.pos >= len(d.data) {
				d.err = fmt.Errorf("invalid dictionary format: %s", d.data)
				return nil
			}

			if d.peekByte() == endOfIdentifier {
				d.readByte()
				break
			}

			key, ok := d.decode().(string)
			if !ok {
				d.err = fmt.Errorf("invalid dictionary format, each key must be string type: %s", d.data)
				return nil
			}

			value := d.decode()
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

		for i := d.pos; i < len(d.data); i++ {
			if d.readByte() == endOfIdentifier {
				endOfNumber = i
				break
			}
		}

		if endOfNumber == 0 {
			d.err = fmt.Errorf("invalid number format: %s", d.data)
			return nil
		}

		number, err := strconv.Atoi(string(d.data[startPos:endOfNumber]))
		if err != nil {
			d.err = err
			return nil
		}

		return number
	case listBencodeIdentifier:
		d.readByte()
		result := []any{}

		for {
			if d.err != nil {
				return nil
			}

			if d.pos >= len(d.data) {
				d.err = fmt.Errorf("invalid list format: %s", d.data)
				return nil
			}

			if d.peekByte() == endOfIdentifier {
				d.readByte()
				break
			}

			val := d.decode()
			result = append(result, val)
		}

		return result
	default:
		initialPos := d.pos
		var firstColonIndex int

		for i := d.pos; i < len(d.data); i++ {
			if d.readByte() == columnIdentifier {
				firstColonIndex = i
				break
			}
		}

		lengthStr := d.data[initialPos:firstColonIndex]

		length, err := strconv.Atoi(string(lengthStr))
		if err != nil {
			d.err = err
			return nil
		}

		if d.pos+length > len(d.data) {
			d.err = fmt.Errorf("invalid string length: %d", length)
			return nil
		}

		d.pos += length
		return string(d.data[firstColonIndex+1 : d.pos])
	}
}

func (d *decoder) readByte() byte {
	b := d.data[d.pos]
	d.pos++
	return b
}

func (d *decoder) peekByte() byte {
	b := d.data[d.pos]
	return b
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		dec := decoder{
			data: []byte(bencodedValue),
		}

		decoded := dec.decode()

		if dec.err != nil {
			fmt.Println(dec.err)
			return
		}

		if _, ok := decoded.(dictionary); ok {
			decoded = decoded.(dictionary).toMap()
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

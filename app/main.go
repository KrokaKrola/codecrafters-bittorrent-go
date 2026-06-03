package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

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

	inputType := d.peekByte()

	switch inputType {
	case dictionaryBencodeIdentifier:
		d.err = fmt.Errorf("dictionary is not supported")
		return nil
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
				break
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

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

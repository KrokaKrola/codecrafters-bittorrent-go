package torrent

import (
	"bufio"
	"fmt"
	"strconv"
)

type readerDecoder struct {
	r   *bufio.Reader
	err error
}

func (d *readerDecoder) decode() any {
	if d.err != nil {
		return nil
	}

	b, err := d.r.ReadByte()
	if err != nil {
		d.err = fmt.Errorf("unexpected end of input")
		return nil
	}

	switch b {
	case dictionaryBencodeIdentifier:
		result := Dictionary{}

		for {
			if d.err != nil {
				return nil
			}

			peek, err := d.r.Peek(1)
			if err != nil {
				d.err = fmt.Errorf("invalid dictionary format: unexpected end")
				return nil
			}

			if peek[0] == endOfIdentifier {
				d.r.ReadByte()
				break
			}

			key, ok := d.decode().(string)
			if !ok {
				d.err = fmt.Errorf("invalid dictionary format, each key must be string type")
				return nil
			}

			value := d.decode()
			result = append(result, dictionaryValue{key, value})
		}

		return result

	case integerBencodeIdentifier:
		var buf []byte

		for {
			b, err := d.r.ReadByte()
			if err != nil {
				d.err = fmt.Errorf("invalid number format: unexpected end")
				return nil
			}

			if b == endOfIdentifier {
				break
			}

			buf = append(buf, b)
		}

		if len(buf) == 0 {
			d.err = fmt.Errorf("invalid number format: empty")
			return nil
		}

		number, err := strconv.Atoi(string(buf))
		if err != nil {
			d.err = err
			return nil
		}

		return number

	case listBencodeIdentifier:
		result := []any{}

		for {
			if d.err != nil {
				return nil
			}

			peek, err := d.r.Peek(1)
			if err != nil {
				d.err = fmt.Errorf("invalid list format: unexpected end")
				return nil
			}

			if peek[0] == endOfIdentifier {
				d.r.ReadByte()
				break
			}

			val := d.decode()
			result = append(result, val)
		}

		return result

	default:
		lenBuf := []byte{b}

		for {
			b, err := d.r.ReadByte()
			if err != nil {
				d.err = fmt.Errorf("invalid string format: unexpected end")
				return nil
			}

			if b == columnIdentifier {
				break
			}

			lenBuf = append(lenBuf, b)
		}

		length, err := strconv.Atoi(string(lenBuf))
		if err != nil {
			d.err = err
			return nil
		}

		buf := make([]byte, length)
		n, err := d.r.Read(buf)
		if err != nil || n != length {
			d.err = fmt.Errorf("invalid string length: %d", length)
			return nil
		}

		return string(buf)
	}
}

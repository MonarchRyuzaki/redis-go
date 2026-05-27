package core

import (
	"bytes"
	"fmt"
	"strconv"
)

const (
	STRING     = '+'
	ERROR      = '-'
	INTEGER    = ':'
	BULK       = '$'
	ARRAY      = '*'
	NULL_BULK  = 'n'
	NULL_ARRAY = 'N'
	RDB_FILE   = 'R'
	STREAMS    = 'S'
)

type Value struct {
	Type  byte
	Str   string
	Num   int
	Bulk  string
	Array []Value
}

func Marshal(v Value) []byte {
	switch v.Type {
	case STRING:
		return []byte(fmt.Sprintf("+%s\r\n", v.Str))
	case ERROR:
		return []byte(fmt.Sprintf("-%s\r\n", v.Str))
	case INTEGER:
		return []byte(fmt.Sprintf(":%d\r\n", v.Num))
	case BULK:
		return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(v.Bulk), v.Bulk))
	case ARRAY:
		var res []byte
		res = append(res, []byte(fmt.Sprintf("*%d\r\n", len(v.Array)))...)
		for _, val := range v.Array {
			res = append(res, Marshal(val)...)
		}
		return res
	case NULL_BULK:
		return []byte("$-1\r\n")
	case NULL_ARRAY:
		return []byte("*-1\r\n")
	case RDB_FILE:
		header := fmt.Sprintf("$%d\r\n", len(v.Bulk))
		return append([]byte(header), []byte(v.Bulk)...)
	case STREAMS:
		var res []byte
		for _, val := range v.Array {
			res = append(res, Marshal(val)...)
		}
		return res
	default:
		return nil
	}
}

func Unmarshal(data []byte) (Value, int, error) {
	if len(data) == 0 {
		return Value{}, 0, fmt.Errorf("empty data")
	}

	_type := data[0]
	switch _type {
	case STRING:
		return unmarshalSimpleString(data)
	case ERROR:
		return unmarshalError(data)
	case INTEGER:
		return unmarshalInt(data)
	case BULK:
		return unmarshalBulk(data)
	case ARRAY:
		return unmarshalArray(data)
	default:
		// Inline command (treat as array of bulks)
		return unmarshalInline(data)
	}
}

func UnmarshalMany(data []byte) ([]Value, int, error) {
	var values []Value
	totalRead := 0
	for totalRead < len(data) {
		v, n, err := Unmarshal(data[totalRead:])
		if err != nil {
			return values, totalRead, err
		}
		values = append(values, v)
		totalRead += n
	}
	return values, totalRead, nil
}

func readLine(data []byte) ([]byte, int, error) {
	for i := 0; i < len(data)-1; i++ {
		if data[i] == '\r' && data[i+1] == '\n' {
			return data[:i], i + 2, nil
		}
	}
	return nil, 0, fmt.Errorf("no CRLF found")
}

func unmarshalSimpleString(data []byte) (Value, int, error) {
	line, n, err := readLine(data[1:])
	if err != nil {
		return Value{}, 0, err
	}
	return Value{Type: STRING, Str: string(line)}, n + 1, nil
}

func unmarshalError(data []byte) (Value, int, error) {
	line, n, err := readLine(data[1:])
	if err != nil {
		return Value{}, 0, err
	}
	return Value{Type: ERROR, Str: string(line)}, n + 1, nil
}

func unmarshalInt(data []byte) (Value, int, error) {
	line, n, err := readLine(data[1:])
	if err != nil {
		return Value{}, 0, err
	}
	num, err := strconv.Atoi(string(line))
	if err != nil {
		return Value{}, 0, err
	}
	return Value{Type: INTEGER, Num: num}, n + 1, nil
}

func unmarshalBulk(data []byte) (Value, int, error) {
	line, n, err := readLine(data[1:])
	if err != nil {
		return Value{}, 0, err
	}
	length, err := strconv.Atoi(string(line))
	if err != nil {
		return Value{}, 0, err
	}

	if length == -1 {
		return Value{Type: NULL_BULK}, n + 1, nil
	}

	totalRead := n + 1
	if len(data) < totalRead+length+2 {
		return Value{}, 0, fmt.Errorf("not enough data for bulk string")
	}

	bulk := data[totalRead : totalRead+length]
	if data[totalRead+length] != '\r' || data[totalRead+length+1] != '\n' {
		return Value{}, 0, fmt.Errorf("missing CRLF after bulk string")
	}

	return Value{Type: BULK, Bulk: string(bulk)}, totalRead + length + 2, nil
}

func unmarshalArray(data []byte) (Value, int, error) {
	line, n, err := readLine(data[1:])
	if err != nil {
		return Value{}, 0, err
	}
	length, err := strconv.Atoi(string(line))
	if err != nil {
		return Value{}, 0, err
	}

	if length == -1 {
		return Value{Type: NULL_ARRAY}, n + 1, nil
	}

	res := Value{Type: ARRAY, Array: make([]Value, 0, length)}
	totalRead := n + 1
	for i := 0; i < length; i++ {
		val, read, err := Unmarshal(data[totalRead:])
		if err != nil {
			return Value{}, 0, err
		}
		res.Array = append(res.Array, val)
		totalRead += read
	}

	return res, totalRead, nil
}

func unmarshalInline(data []byte) (Value, int, error) {
	line, n, err := readLine(data)
	if err != nil {
		return Value{}, 0, err
	}

	parts := bytes.Split(line, []byte(" "))
	var array []Value
	for _, part := range parts {
		if len(part) > 0 {
			array = append(array, Value{Type: BULK, Bulk: string(part)})
		}
	}

	return Value{Type: ARRAY, Array: array}, n, nil
}

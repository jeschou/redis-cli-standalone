package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

type RType byte

const TypeSimpleString RType = '+'
const TypeError RType = '-'
const TypeInt RType = ':'
const TypeBulkString RType = '$'
const TypeArray RType = '*'

// redis data type and value
type TypedVal struct {
	Type RType
	Val  any // real type may be string, int, []*TypedVal, nil
}

// read typed value from stream, base on redis protocol
func ReadValue(bufReader *bufio.Reader) (res *TypedVal, err error) {
	res = &TypedVal{}
	typ, err := bufReader.ReadByte()
	if err != nil {
		return
	}
	res.Type = RType(typ)
	var result []byte
	switch res.Type {
	case TypeSimpleString: // simple string
		result, _, err = bufReader.ReadLine()
		res.Val = string(result)
		return
	case TypeError: // err
		result, _, err = bufReader.ReadLine()
		res.Val = string(result)
		return
	case TypeInt: // integer
		result, _, err = bufReader.ReadLine()
		res.Val, _ = strconv.Atoi(string(result))
		return
	case TypeBulkString: // bulk string
		result, _, err = bufReader.ReadLine()
		length, _ := strconv.Atoi(string(result))
		if length == -1 {
			res.Val = nil
		} else if length == 0 {
			res.Val = ""
			_, _, err = bufReader.ReadLine()
		} else {
			result = make([]byte, length)
			_, err = io.ReadAtLeast(bufReader, result, length)
			if err != nil {
				return
			}
			res.Val = string(result)
			_, _, err = bufReader.ReadLine()
		}
		return
	case TypeArray: // array
		var count int
		result, _, err = bufReader.ReadLine()
		count, _ = strconv.Atoi(string(result))
		res0 := make([]*TypedVal, count)
		for i := 0; i < count; i++ {
			v, err := ReadValue(bufReader)
			if err != nil {
				return nil, err
			}
			res0[i] = v
		}
		res.Val = res0
		return
	default:
		err = fmt.Errorf("unknown response type: %c", res.Type)
	}
	return
}

// convert typed value to string and print to writer
// compatible with redis-cli
func PrintVal(writer io.Writer, res *TypedVal, raw bool) {
	if res.Val == nil {
		if raw {
			_, _ = fmt.Fprintf(writer, "\n")
		} else {
			_, _ = fmt.Fprintf(writer, "(nil)\n")
		}
	} else {
		switch res.Type {
		case TypeSimpleString:
			_, _ = fmt.Fprintf(writer, "%s\n", res.Val)
		case TypeBulkString:
			if raw {
				_, _ = fmt.Fprintf(writer, "%s\n", res.Val)
			} else {
				_, _ = fmt.Fprintf(writer, "%q\n", res.Val)
			}
		case TypeError:
			if raw {
				_, _ = fmt.Fprintf(writer, "%s\n", res.Val)
			} else {
				_, _ = fmt.Fprintf(writer, "(error) %s\n", res.Val)
			}
		case TypeInt:
			if raw {
				_, _ = fmt.Fprintf(writer, "%d\n", res.Val)
			} else {
				_, _ = fmt.Fprintf(writer, "(integer) %d\n", res.Val)
			}
		case TypeArray:
			for i, v := range res.Val.([]*TypedVal) {
				if !raw {
					_, _ = fmt.Fprintf(writer, "%d) ", i+1)
				}
				PrintVal(writer, v, raw)
			}
		}
	}
}

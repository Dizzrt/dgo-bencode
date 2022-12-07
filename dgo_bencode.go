package dgoBencode

import (
	"bufio"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
)

var (
	ErrNum    = errors.New("expect num")
	ErrFormat = errors.New("wrong bencode format")
)

func writeDecimal(w *bufio.Writer, val int) (length int) {
	if val == 0 {
		w.WriteByte('0')
		length++
		return
	}
	if val < 0 {
		w.WriteByte('-')
		length++
		val *= -1
	}

	divisor := 1
	for {
		if divisor > val {
			divisor /= 10
			break
		}
		divisor *= 10
	}

	for {
		num := byte(val / divisor)
		w.WriteByte('0' + num)
		length++

		if divisor == 1 {
			break
		}

		val %= divisor
		divisor /= 10
	}

	return
}

func readDecimal(r *bufio.Reader) (res int, length int) {
	isNegative := false
	temp, _ := r.ReadByte()
	length++

	if temp == '-' {
		isNegative = true
		temp, _ = r.ReadByte()
		length++
	}

	for {
		if temp < '0' || temp > '9' {
			r.UnreadByte()
			length--
			break
		}

		res = res*10 + int(temp-'0')
		temp, _ = r.ReadByte()
		length++
	}

	if isNegative {
		res = -res
	}
	return
}

func encodeString(w io.Writer, str string) int {
	strLength := len(str)
	bw := bufio.NewWriter(w)
	wlen := writeDecimal(bw, strLength)
	bw.WriteByte(':')
	wlen++
	bw.WriteString(str)
	wlen += strLength

	err := bw.Flush()
	if err != nil {
		return 0
	}
	return wlen
}

func decodeString(r io.Reader) (str string, err error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}

	num, len := readDecimal(br)
	if len == 0 {
		return str, ErrNum
	}

	b, err := br.ReadByte()
	if b != ':' {
		return str, ErrFormat
	}

	buf := make([]byte, num)
	_, err = io.ReadAtLeast(br, buf, num)
	str = string(buf)
	return
}

func encodeInt(w io.Writer, val int) int {
	bw := bufio.NewWriter(w)
	wlen := 0

	bw.WriteByte('i')
	wlen++

	wlen += writeDecimal(bw, val)

	bw.WriteByte('e')
	wlen++

	err := bw.Flush()
	if err != nil {
		return 0
	}
	return wlen
}

func decodeInt(r io.Reader) (val int, err error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}

	temp, err := br.ReadByte()
	if temp != 'i' {
		return val, ErrFormat
	}

	val, _ = readDecimal(br)
	temp, err = br.ReadByte()
	if temp != 'e' {
		return val, ErrFormat
	}

	return
}

func encodeSlice(w io.Writer, arr []any) int {
	bw := bufio.NewWriter(w)
	wlen := 0

	bw.WriteByte('l')
	wlen++

	for _, element := range arr {
		kindOfElement := reflect.TypeOf(element).Kind()
		v := reflect.ValueOf(element).Interface()

		switch kindOfElement {
		case reflect.Int:
			wlen += encodeInt(bw, v.(int))
		case reflect.String:
			wlen += encodeString(bw, v.(string))
		case reflect.Slice:
			wlen += encodeSlice(bw, v.([]any))
		case reflect.Map:
			wlen += encodeMap(bw, v.(map[string]any))
		}
	}

	bw.WriteByte('e')
	wlen++

	err := bw.Flush()
	if err != nil {
		return 0
	}
	return wlen
}

func decodeSlice(r io.Reader) (arr []any, err error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}

	temp, err := br.ReadByte()
	if temp != 'l' {
		return arr, ErrFormat
	}

	for {
		temp, _ := br.ReadByte()
		br.UnreadByte()

		if temp == 'i' {
			val, err := decodeInt(br)
			if err != nil {
				return arr, err
			}
			arr = append(arr, val)
		} else if temp >= '0' && temp <= '9' {
			str, err := decodeString(br)
			if err != nil {
				return arr, err
			}
			arr = append(arr, str)
		} else if temp == 'l' {
			subArr, err := decodeSlice(br)
			if err != nil {
				return arr, err
			}
			arr = append(arr, subArr)
		} else if temp == 'd' {
			//TODO decode map
		} else if temp == 'e' {
			br.ReadByte()
			break
		} else {
			return arr, ErrFormat
		}
	}

	return
}

func encodeMap(w io.Writer, dict map[string]any) int {
	bw := bufio.NewWriter(w)
	wlen := 0

	bw.WriteByte('d')
	wlen++

	keys := make([]string, 0, len(dict))
	for k := range dict {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := dict[k]

		wlen += encodeString(bw, k)
		kindOfElement := reflect.TypeOf(v).Kind()

		vv := reflect.ValueOf(v).Interface()
		switch kindOfElement {
		case reflect.Int:
			wlen += encodeInt(bw, vv.(int))
		case reflect.String:
			wlen += encodeString(bw, vv.(string))
		case reflect.Slice:
			wlen += encodeSlice(bw, vv.([]any))
		case reflect.Map:
			wlen += encodeMap(bw, vv.(map[string]any))
		}
	}

	bw.WriteByte('e')
	wlen++

	err := bw.Flush()
	if err != nil {
		return 0
	}

	return wlen
}

func decodeMap(r io.Reader) (dict map[string]any, err error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	dict = make(map[string]any)

	temp, err := br.ReadByte()
	if temp != 'd' {
		return dict, ErrFormat
	}

	for {
		temp, _ := br.ReadByte()
		br.UnreadByte()

		if temp == 'e' {
			br.ReadByte()
			break
		}

		key, err := decodeString(br)
		if err != nil {
			return dict, err
		}

		temp, _ = br.ReadByte()
		br.UnreadByte()

		if temp == 'i' {
			val, err := decodeInt(br)
			if err != nil {
				return dict, err
			}
			dict[key] = val
		} else if temp >= '0' && temp <= '9' {
			str, err := decodeString(br)
			if err != nil {
				return dict, err
			}
			dict[key] = str
		} else if temp == 'l' {
			sli, err := decodeSlice(br)
			if err != nil {
				return dict, err
			}
			dict[key] = sli
		} else if temp == 'd' {
			subDict, err := decodeMap(br)
			if err != nil {
				return dict, err
			}
			dict[key] = subDict
		} else {
			return dict, ErrFormat
		}
	}

	return
}

func BencodeEncode(w io.Writer, obj any) (err error) {
	kindOfObj := reflect.TypeOf(obj).Kind()
	switch kindOfObj {
	case reflect.Int:
		encodeInt(w, obj.(int))
	case reflect.String:
		encodeString(w, obj.(string))
	case reflect.Slice:
		encodeSlice(w, obj.([]any))
	case reflect.Map:
		encodeMap(w, obj.(map[string]any))
	}
	return nil
}

func BencodeDecodeFromReader(r io.Reader) (objs []any, err error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}

	for {
		temp, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		br.UnreadByte()

		if temp == 'i' {
			num, err := decodeInt(br)
			if err != nil {
				return objs, err
			}
			objs = append(objs, num)
		} else if temp >= '0' && temp <= '9' {
			str, err := decodeString(br)
			if err != nil {
				return objs, err
			}
			objs = append(objs, str)
		} else if temp == 'l' {
			sli, err := decodeSlice(br)
			if err != nil {
				return objs, err
			}
			objs = append(objs, sli)
		} else if temp == 'd' {
			dict, err := decodeMap(br)
			if err != nil {
				return objs, err
			}
			objs = append(objs, dict)
		} else {
			return objs, ErrFormat
		}
	}

	return
}

func BencodeDecodeFromString(str string) (objs []any, err error) {
	return BencodeDecodeFromReader(strings.NewReader(str))
}

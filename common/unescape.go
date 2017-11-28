package common

import (
	"bytes"
	"strconv"
)

const UnescapeCommonPattern = `\\([nftbrv\"\\]|[0-7]{1,3}|x[0-9a-fA-F]{2}|u[0-9a-fA-F]{4}|U[0-9a-fA-F]{8})`

func UnescapeCommon(esc string, buf *bytes.Buffer) bool {
	if len(esc) < 2 || esc[0] != '\\' {
		return false
	}

	switch esc[1] {
	case '"':
		buf.WriteByte('"')
	case '\\':
		buf.WriteByte('\\')
	case 'n':
		buf.WriteByte('\n')
	case 't':
		buf.WriteByte('\t')
	case 'r':
		buf.WriteByte('\r')
	case 'f':
		buf.WriteByte('\f')
	case 'b':
		buf.WriteByte('\b')
	case 'v':
		buf.WriteByte('\v')
	case '0', '1', '2', '3', '4', '5', '6', '7':
		if i, err := strconv.ParseInt(esc[1:], 8, 8); err != nil {
			buf.WriteByte(byte(i))
		} else {
			return false
		}
	case 'x':
		if i, err := strconv.ParseInt(esc[2:], 16, 8); err != nil {
			buf.WriteByte(byte(i))
		} else {
			return false
		}
	case 'u', 'U':
		if i, err := strconv.ParseInt(esc[2:], 16, 32); err != nil {
			buf.WriteRune(rune(i))
		} else {
			return false
		}
	default:
		return false
	}

	return true
}

package jsondiff

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"
)

const (
	separator   = '/'
	emptyPtr    = ""
	escapeSlash = "~1"
	escapeTilde = "~0"
)

var (
	// rfc6901Escaper is a replacer that escapes a JSON
	// Pointer string in compliance with the JavaScript
	// Object Notation Pointer syntax.
	// https://tools.ietf.org/html/rfc6901
	rfc6901Escaper = strings.NewReplacer("~", "~0", "/", "~1")

	// pointerToGJSONPath converts a RFC6901 JSON Pointer to a GJSON Path.
	// See https://github.com/tidwall/gjson/blob/master/SYNTAX.md
	pointerToGJSONPath = strings.NewReplacer(".", "\\.", "*", "\\*", "?", "\\?", "/", ".", "~0", "~", "~1", "/")
)

// pointer represents an RFC 6901 JSON Pointer.
type pointer struct {
	buf []byte
	end int
}

func (p pointer) string() string {
	return *(*string)(unsafe.Pointer(&p.buf))
}

func (p pointer) copy() string {
	return string(p.buf)
}

func (p pointer) base() string {
	b := p.buf[p.end+1:]
	return *(*string)(unsafe.Pointer(&b))
}

func (p pointer) appendKey(key string) pointer {
	p.buf = append(p.buf, separator)
	return p.appendEscapeKey(key)
}

func (p pointer) appendIndex(idx int) pointer {
	p.buf = append(p.buf, separator)
	p.buf = strconv.AppendInt(p.buf, int64(idx), 10)
	return p
}

func (p pointer) snapshot() pointer {
	return pointer{
		buf: p.buf,
		end: len(p.buf),
	}
}

func (p pointer) rewind() pointer {
	return pointer{
		buf: p.buf[:p.end],
		end: p.end,
	}
}

func (p pointer) appendEscapeKey(k string) pointer {
	for _, r := range k {
		if r == '/' {
			p.buf = append(p.buf, escapeSlash...)
			continue
		} else if r == '~' {
			p.buf = append(p.buf, escapeTilde...)
			continue
		}
		p.buf = utf8.AppendRune(p.buf, r)
	}
	return p
}

func (p pointer) isRoot() bool {
	return len(p.buf) == 0
}

func (p pointer) reset() pointer {
	p.buf = p.buf[:0]
	p.end = 0
	return p
}

func toJSONPath(s string) string {
	if len(s) != 0 {
		return pointerToGJSONPath.Replace(s[1:])
	}
	// @this is a special modifier that can
	// be used to retrieve the root path.
	return "@this"
}

var (
	errLeadingSlash             = errors.New("no leading slash")
	errIncompleteEscapeSequence = errors.New("incomplete escape sequence")
	errInvalidEscapeSequence    = errors.New("invalid escape sequence")
)

func parsePointer(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	a := []rune(s)

	if len(a) > 0 && a[0] != '/' {
		return nil, errLeadingSlash
	}
	var tokens []string

	ls := 0
	for i, r := range a {
		if r == '/' {
			if i != 0 {
				tokens = append(tokens, string(a[ls+1:i]))
			}
			if i == len(a)-1 {
				// Last char is a '/', next fragment is an empty string.
				tokens = append(tokens, "")
				break
			}
			ls = i
		} else if r == '~' {
			if i == len(a)-1 {
				return nil, errIncompleteEscapeSequence
			}
			if a[i+1] != '0' && a[i+1] != '1' {
				return nil, errInvalidEscapeSequence
			}
		} else {
			if i == len(a)-1 {
				// End of string, accumulate from last separator.
				tokens = append(tokens, string(a[ls+1:]))
			}
		}
	}
	return tokens, nil
}

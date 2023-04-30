package jsondiff

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	ptrSeparator = "/"
	emptyPtr     = pointer("")
)

var (
	// rfc6901Escaper is a replacer that escapes a JSON
	// Pointer string in compliance with the JavaScript
	// Object Notation Pointer syntax.
	// https://tools.ietf.org/html/rfc6901
	rfc6901Escaper = strings.NewReplacer("~", "~0", "/", "~1")

	// dotPathReplacer converts a RFC6901 JSON pointer to
	// a JSON path, while also escaping any existing dot
	// characters present in the original pointer.
	dotPathReplacer = strings.NewReplacer(".", "\\.", "/", ".")
)

// pointer represents a RFC6901 JSON Pointer.
type pointer string

// String implements the fmt.Stringer interface.
func (p pointer) String() string {
	return string(p)
}

func (p pointer) toJSONPath() string {
	if len(p) > 0 {
		return dotPathReplacer.Replace(string(p)[1:])
	}
	// @this is a special modifier that can
	// be used to retrieve the root path.
	return "@this"
}

func (p pointer) appendKey(key string) pointer {
	return pointer(string(p) + ptrSeparator + rfc6901Escaper.Replace(key))
}

func (p pointer) appendIndex(idx int) pointer {
	return pointer(string(p) + ptrSeparator + strconv.Itoa(idx))
}

func (p pointer) isRoot() bool {
	return len(p) == 0
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
			if !isUnescaped(r) {
				return nil, fmt.Errorf("invalid rune %q", r)
			}
			if i == len(a)-1 {
				// End of string, accumulate from last separator.
				tokens = append(tokens, string(a[ls+1:]))
			}
		}
	}
	return tokens, nil
}

func isUnescaped(r rune) bool {
	// Unescaped range is defined as:
	// %x00-2E / %x30-7D / %x7F-10FFFF
	// https://datatracker.ietf.org/doc/html/rfc6901#section-3
	return r >= 0x00 && r <= 0x2E || r >= 0x30 && r <= 0x7D || r >= 0x7F && r <= 0x10FFFF
}

package jsondiff

import (
	"strconv"
	"strings"
)

const separator = "/"

// rfc6901Replacer is a replacer used to escape JSON
// pointer strings in compliance with the JavaScript
// Object Notation Pointer syntax.
// https://tools.ietf.org/html/rfc6901
var rfc6901Replacer = strings.NewReplacer("~", "~0", "/", "~1")

type jsonNode struct {
	ptr pointer
	val interface{}
}

// pointer represents a RFC6901 JSON Pointer.
type pointer string

const emptyPtr = pointer("")

// String implements the fmt.Stringer interface.
func (p pointer) String() string {
	return string(p)
}

func (p pointer) appendKey(key string) pointer {
	return pointer(p.String() + separator + rfc6901Replacer.Replace(key))
}

func (p pointer) appendIndex(idx int) pointer {
	return pointer(p.String() + separator + strconv.Itoa(idx))
}

func (p pointer) isRoot() bool {
	return len(p) == 0
}

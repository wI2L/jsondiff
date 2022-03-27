package jsondiff

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
)

// JSON Patch operation types.
// These are defined in RFC 6902 section 4.
// https://datatracker.ietf.org/doc/html/rfc6902#section-4
const (
	OperationAdd     = "add"
	OperationReplace = "replace"
	OperationRemove  = "remove"
	OperationMove    = "move"
	OperationCopy    = "copy"
	OperationTest    = "test"
)

const (
	operationBase = `{"op":"","path":""}`
	fromPrefix    = `,"from":""`
	valuePrefix   = `,"value":`
)

// Operation represents a RFC6902 JSON Patch operation.
type Operation struct {
	Type     string      `json:"op"`
	From     pointer     `json:"from,omitempty"`
	Path     pointer     `json:"path"`
	OldValue interface{} `json:"-"`
	Value    interface{} `json:"value,omitempty"`
}

// String implements the fmt.Stringer interface.
func (o Operation) String() string {
	b, err := json.Marshal(o)
	if err != nil {
		return "<invalid operation>"
	}
	return string(b)
}

// MarshalJSON implements the json.Marshaler interface.
func (o Operation) MarshalJSON() ([]byte, error) {
	type op Operation
	switch o.Type {
	case OperationCopy, OperationMove:
		o.Value = nil
	case OperationAdd, OperationReplace, OperationTest:
		o.From = emptyPtr
	}
	return json.Marshal(op(o))
}

// jsonLength returns the length in bytes that the
// operation would occupy when marshaled as JSON.
func (o Operation) jsonLength(targetBytes []byte) int {
	l := len(operationBase) + len(o.Type) + len(o.Path)

	if o.Type != OperationCopy && o.Type != OperationMove {
		var valueLen int
		if o.Path.isRoot() {
			valueLen = len(targetBytes)
		} else {
			r := gjson.GetBytes(targetBytes, o.Path.toJSONPath())
			valueLen = len(r.Raw)
		}
		l += len(valuePrefix) + valueLen
	}
	if o.Type != OperationAdd && o.Type != OperationReplace && o.Type != OperationTest {
		l += len(fromPrefix) + len(o.From)
	}
	return l
}

// Patch represents a series of JSON Patch operations.
type Patch []Operation

// String implements the fmt.Stringer interface.
func (p Patch) String() string {
	sb := strings.Builder{}

	for i, op := range p {
		if i != 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(op.String())
	}
	return sb.String()
}

func (p *Patch) remove(idx int) Patch {
	return (*p)[:idx+copy((*p)[idx:], (*p)[idx+1:])]
}

func (p *Patch) append(typ string, from, path pointer, src, tgt interface{}) Patch {
	return append(*p, Operation{
		Type:     typ,
		From:     from,
		Path:     path,
		OldValue: src,
		Value:    tgt,
	})
}

func (p Patch) jsonLength(targetBytes []byte) int {
	length := 0
	for _, op := range p {
		length += op.jsonLength(targetBytes)
	}
	// Count comma-separators if the patch
	// has more than one operation.
	if len(p) > 1 {
		length += len(p) - 1
	}
	return length
}

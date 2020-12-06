package jsondiff

import (
	"encoding/json"
	"strings"
)

const (
	OperationAdd     = "add"
	OperationReplace = "replace"
	OperationRemove  = "remove"
	OperationMove    = "move"
	OperationCopy    = "copy"
	OperationTest    = "test"
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

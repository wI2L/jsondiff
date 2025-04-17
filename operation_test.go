package jsondiff

import (
	"testing"
)

func TestOperation_MarshalJSON(t *testing.T) {
	for _, tc := range []struct {
		Op  Operation
		Out string // marshaled output
	}{
		// Replace and add operations should ALWAYS be marshaled
		// with a value, even if it is null (override omitempty).
		{
			Operation{
				Type:  OperationReplace,
				Path:  "/foo/bar",
				Value: nil,
			},
			`{"value":null,"op":"replace","path":"/foo/bar"}`,
		},
		{
			Operation{
				Type:  OperationReplace,
				Path:  "/foo/bar",
				Value: typeNilIface(),
			},
			`{"value":null,"op":"replace","path":"/foo/bar"}`,
		},
		{
			Operation{
				Type:  OperationReplace,
				Path:  "/foo/bar",
				Value: "foo",
			},
			`{"value":"foo","op":"replace","path":"/foo/bar"}`,
		},
		{
			// assigned interface
			Operation{
				Type:  OperationAdd,
				Path:  "",
				Value: nil,
			},
			`{"value":null,"op":"add","path":""}`,
		},
		{
			// unassigned interface Value
			Operation{
				Type: OperationAdd,
				Path: "",
			},
			`{"value":null,"op":"add","path":""}`,
		},
		{
			Operation{
				Type:  OperationAdd,
				Path:  "",
				Value: typeNilIface(),
			},
			`{"value":null,"op":"add","path":""}`,
		},
		{
			// Remove operation should NEVER be marshaled with a value.
			Operation{
				Type:  OperationRemove,
				Path:  "/foo/bar",
				Value: 0,
			},
			`{"op":"remove","path":"/foo/bar"}`,
		},
		{
			// Copy operation should NEVER be marshaled with a value.
			Operation{
				Type:  OperationCopy,
				From:  "/bar",
				Path:  "/baz",
				Value: 0,
			},
			`{"op":"copy","from":"/bar","path":"/baz"}`,
		},
		{
			// Move operation should NEVER be marshaled with a value.
			Operation{
				Type:  OperationMove,
				From:  "/bar",
				Path:  "/baz",
				Value: 0,
			},
			`{"op":"move","from":"/bar","path":"/baz"}`,
		},
	} {
		b, err := tc.Op.MarshalJSON()
		if err != nil {
			t.Errorf("failed to marshal operation: %s", err)
		}
		if tc.Out != string(b) {
			t.Errorf("marshaled patch mismatched, got %q, want %q", string(b), tc.Out)
		}
	}
}

func TestPatch_String(t *testing.T) {
	patch := Patch{
		{
			Type:  OperationReplace,
			Path:  "/foo/baz",
			Value: 42,
		},
		{
			Type: OperationRemove,
			Path: "/xxx",
		},
		{
			Type:  OperationAdd,
			Path:  "/zzz",
			Value: make(chan<- string), // UnsupportedTypeError
		},
	}
	s := patch.String()

	const expected = `{"value":42,"op":"replace","path":"/foo/baz"}
{"op":"remove","path":"/xxx"}
<invalid operation>`

	if s != expected {
		t.Errorf("stringified operation mismatch, got %q, want %q", s, expected)
	}
}

func TestPatch_String_nil(t *testing.T) {
	p := new(Patch)
	s := p.String()

	if s != "" {
		t.Errorf("stringified operation mismatch, got %q, want empty string", s)
	}
}

func TestPatch_jsonLength(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		p := new(Patch)
		l := p.jsonLength()
		if l != 0 {
			t.Error("expected zero length for empty patch")
		}
	})
	t.Run("nil", func(t *testing.T) {
		var p *Patch
		l := p.jsonLength()
		if l != 0 {
			t.Error("expected zero length for nil patch")
		}
	})
}

func Test_issue26(_ *testing.T) {
	p := new(Patch)
	p.insert(6, OperationAdd, emptyPointer, "/a", nil, 42, 0)
}

func typeNilIface() interface{} {
	var i *int
	var p interface{} //nolint:staticcheck

	p = i

	return p
}

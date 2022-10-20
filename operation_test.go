package jsondiff

import "testing"

func TestOperationMarshalJSON(t *testing.T) {
	for _, tc := range []struct {
		Op  Operation
		Out string // marshaled output
	}{
		// Replace operations should ALWAYS be marshaled
		// with a value, even if it is null (override omitempty).
		{
			Operation{
				Type:  OperationReplace,
				Path:  "/foo/bar",
				Value: nil,
			},
			`{"op":"replace","path":"/foo/bar","value":null}`,
		},
		{
			Operation{
				Type:  OperationReplace,
				Path:  "/foo/bar",
				Value: "foo",
			},
			`{"op":"replace","path":"/foo/bar","value":"foo"}`,
		},
		{
			// Remove operation should NEVER be marshaled with
			// a value.
			Operation{
				Type:  OperationRemove,
				Path:  "/foo/bar",
				Value: 0,
			},
			`{"op":"remove","path":"/foo/bar"}`,
		},
		{
			// Copy operation should NEVER be marshaled with
			// a value.
			Operation{
				Type:  OperationCopy,
				From:  "/bar",
				Path:  "/baz",
				Value: 0,
			},
			`{"op":"copy","from":"/bar","path":"/baz"}`,
		},
		{
			// Move operation should NEVER be marshaled with
			// a value.
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

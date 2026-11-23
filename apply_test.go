package jsondiff

import (
	"testing"
)

func Test_toDotPath(t *testing.T) {
	for _, tc := range []struct {
		ptr  string
		json string
		path string
	}{
		{
			"",
			`{}`,
			"@this",
		},
		{
			"/a/b/c",
			`{"a":{"b":{"c":1}}}`,
			"a.b.c",
		},
		{
			"/a/1/c",
			`{"a":[null,{"c":1}]}`,
			"a.1.c",
		},
		{
			"/a/123/b",
			`{"a":{"123":{"b":1"}}}`,
			"a.:123.b",
		},
		{
			"/1",
			`["a","b","c"]`,
			"1",
		},
		{
			"/0",
			`{"0":"a"}`,
			":0",
		},
		{
			"/a/-",
			`{"a":[1,2,3]}`,
			"a.-1",
		},
	} {
		s, err := toDotPath(tc.ptr, []byte(tc.json))
		if err != nil {
			t.Error(err)
		}
		if s != tc.path {
			t.Errorf("got %q, want %q", s, tc.path)
		}
	}
}

func Test_isArrayIndex(t *testing.T) {
	for _, tc := range []struct {
		path    string
		isIndex bool
	}{
		{"a.b.c", false},
		{"", false},
		{"a.b.:124", false},
		{"a.-1", false},
		{"0.1.a", false},
		{"0.1.2.:3", false},
		{"a\\.b", false},
		{"0.1\\.2", false},
		{"0", true},
		{"a.b.1", true},
		{"0.1.2", true},
	} {
		b := isArrayIndex(tc.path)
		if tc.isIndex && !b {
			t.Errorf("expected path %q to be an array index", tc.path)
		}
		if !tc.isIndex && b {
			t.Errorf("expected path %q to not be an array index", tc.path)
		}
	}
}

func TestPatch_apply_invalidJSON(t *testing.T) {
	badJSON := []byte(`{"example":2:]}}`)

	var p Patch
	if _, err := p.apply(badJSON, true); err == nil {
		t.Errorf("expected non-nil error")
	}
}

func TestPatch_apply_moveError(t *testing.T) {
	src := []byte(`[1,2,3]`)

	// sjson refuses to pad an array past 100000 elements,
	// so the add operation of the moved value fails.
	// The error used to be swallowed, leaving the source deleted
	// and the destination never added.
	p := Patch{
		{Type: OperationMove, From: "/0", Path: "/200000"},
	}
	if _, err := p.apply(src, true); err == nil {
		t.Error("expected non-nil error")
	}
}

package jsondiff

import (
	"reflect"
	"testing"
)

func Test_parsePointer(t *testing.T) {
	for _, tt := range []struct {
		ptr    string
		valid  bool
		err    error
		ntok   int
		tokens []string
	}{
		// RFC Section 5.
		// https://tools.ietf.org/html/rfc6901#section-5
		{
			"",
			true,
			nil,
			0,
			nil,
		},
		{
			"/foo",
			true,
			nil,
			1,
			[]string{"foo"},
		},
		{
			"/foo/0",
			true,
			nil,
			2,
			[]string{"foo", "0"},
		},
		{
			"/",
			true,
			nil,
			1,
			[]string{""},
		},
		{
			"/a~1b",
			true,
			nil,
			1,
			[]string{"a~1b"},
		},
		{
			"/c%d",
			true,
			nil,
			1,
			[]string{"c%d"},
		},
		{
			"/e^f",
			true,
			nil,
			1,
			[]string{"e^f"},
		},
		{
			"/g|h",
			true,
			nil,
			1,
			[]string{"g|h"},
		},
		{
			"/i\\j",
			true,
			nil,
			1,
			[]string{"i\\j"},
		},
		{
			"/k\"l",
			true,
			nil,
			1,
			[]string{"k\"l"},
		},
		{
			"/ ",
			true,
			nil,
			1,
			[]string{" "},
		},
		{
			"/m~0n",
			true,
			nil,
			1,
			[]string{"m~0n"},
		},
		// Custom tests.
		// Simple.
		{
			"/a/b/c",
			true,
			nil,
			3,
			[]string{"a", "b", "c"},
		},
		{
			"/a/0/b",
			true,
			nil,
			3,
			[]string{"a", "0", "b"},
		},
		// Complex.
		{
			"/a/b/",
			true,
			nil,
			3,
			[]string{"a", "b", ""},
		},
		// Error cases.
		{
			"a/b/c",
			false,
			errLeadingSlash,
			0,
			nil,
		},
		{
			"/a/~",
			false,
			errIncompleteEscapeSequence,
			0,
			nil,
		},
		{
			"/a/b/~3",
			false,
			errInvalidEscapeSequence,
			0,
			nil,
		},
	} {
		tokens, err := parsePointer(tt.ptr)
		if tt.valid && err != nil {
			t.Errorf("expected valid pointer, got error: %q", err)
		}
		if !tt.valid {
			if err == nil {
				t.Errorf("expected error, got none")
			} else if err != tt.err {
				t.Errorf("error mismtahc, got %q, want %q", err, tt.err)
			}
		}
		if l := len(tokens); l != tt.ntok {
			t.Errorf("got %d tokens, want %d: %q", l, tt.ntok, tt.ptr)
		} else {
			if !reflect.DeepEqual(tokens, tt.tokens) {
				t.Errorf("tokens mismatch, got %v, want %v", tokens, tt.tokens)
			}
		}
	}
}

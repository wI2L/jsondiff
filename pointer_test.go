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
		count  int
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
		if l := len(tokens); l != tt.count {
			t.Errorf("got %d tokens, want %d: %q", l, tt.count, tt.ptr)
		} else {
			if !reflect.DeepEqual(tokens, tt.tokens) {
				t.Errorf("tokens mismatch, got %v, want %v", tokens, tt.tokens)
			}
		}
	}
}

func BenchmarkEscapeKey(b *testing.B) {
	if testing.Short() {
		b.Skip()
	}
	const key = "a/b~x~1!~0"

	b.Run("strings.Replacer", func(b *testing.B) {
		p := pointer{buf: make([]byte, 0, len(key)*2)}
		for i := 0; i < b.N; i++ {
			p.buf = append(p.buf, rfc6901Escaper.Replace(key)...)
			p.buf = p.buf[:0]
		}
	})
	b.Run("appendEscapeKey", func(b *testing.B) {
		p := pointer{buf: make([]byte, 0, len(key)*2)}
		for i := 0; i < b.N; i++ {
			p.appendEscapeKey(key)
			p.buf = p.buf[:0]
		}
	})
}

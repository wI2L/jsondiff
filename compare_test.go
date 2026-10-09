package jsondiff

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestCompareJSON_rationalizeEscapedStrings(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		target string
	}{
		{"object", `{"a":"old","b":1}`, `{"a":"\\","b":2}`},
		{"array", `["old",1]`, `["\\",2]`},
		{"nested", `{"a":{"value":"old","next":1},"b":1}`, `{"a":{"value":"\\\\","next":2},"b":2}`},
	} {
		for _, mode := range []struct {
			name string
			opts []Option
		}{
			{"compact", []Option{Rationalize()}},
			{"skip_compact", []Option{Rationalize(), SkipCompact()}},
			{"in_place", []Option{Rationalize(), InPlaceCompaction()}},
			{"invertible_lcs", []Option{Rationalize(), Invertible(), LCS()}},
		} {
			t.Run(tc.name+"/"+mode.name, func(t *testing.T) {
				source, target := []byte(tc.source), []byte(tc.target)
				patch, err := CompareJSON(source, target, mode.opts...)
				if err != nil {
					t.Fatal(err)
				}
				got, err := patch.apply(source, true)
				if err != nil {
					t.Fatal(err)
				}
				if !jsonBytesEqual(t, got, target) {
					t.Errorf("patch produced %s, want %s", got, target)
				}
			})
		}
	}
}

func Test_marshalUnmarshal_invalid_JSON(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		e := errors.New("marshal")
		_, _, err := marshalUnmarshal(nil, options{
			// Override marshal func to simulate error.
			marshal:   func(any) ([]byte, error) { return nil, e },
			unmarshal: json.Unmarshal,
		})
		if !errors.Is(err, e) {
			t.Errorf("expected non-nil error")
		}
	})
	t.Run("unmarshal", func(t *testing.T) {
		e := errors.New("unmarshal")
		_, _, err := marshalUnmarshal(nil, options{
			// Override unmarshal func to simulate error.
			marshal:   json.Marshal,
			unmarshal: func([]byte, any) error { return e },
		})
		if !errors.Is(err, e) {
			t.Errorf("expected non-nil error")
		}
	})
}

func TestCompareWithoutMarshal_error(t *testing.T) {
	type custom struct {
		foo string
		bar int
	}
	_, err := CompareWithoutMarshal(custom{"a", 1}, custom{"b", 2})
	if err == nil {
		t.Errorf("expected non-nil error")
	}
	t.Log(err)
}

type skip struct{}

var skipBytes = []byte("skip")

func Test_compare_marshaling_error(t *testing.T) {
	e := errors.New("")

	d := Differ{opts: options{
		marshal: func(a any) ([]byte, error) {
			if _, ok := a.(skip); ok {
				return skipBytes, nil
			}
			return nil, e
		},
		unmarshal: func(b []byte, _ any) error {
			if bytes.Equal(b, skipBytes) {
				return nil
			}
			return e
		},
	}}
	if _, err := compare(&d, nil, nil); !errors.Is(err, e) {
		t.Errorf("expected non-nil error")
	}
	if _, err := compare(&d, skip{}, nil); !errors.Is(err, e) {
		t.Errorf("expected non-nil error")
	}
}

func Test_compareJSON_marshaling_error(t *testing.T) {
	e := errors.New("")

	unmarshalFn := func(b []byte, _ any) error {
		if bytes.Equal(b, skipBytes) {
			return nil
		}
		return e
	}
	if _, err := compareJSON(nil, nil, nil, unmarshalFn); !errors.Is(err, e) {
		t.Errorf("expected non-nil error")
	}
	if _, err := compareJSON(nil, []byte("skip"), nil, unmarshalFn); !errors.Is(err, e) {
		t.Errorf("expected non-nil error")
	}
}

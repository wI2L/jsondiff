package jsondiff

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

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

func TestCompareJSONEquivalentPatchRoundTrip(t *testing.T) {
	// With Equivalent enabled, the hasher used to sort arrays in place to compute
	// an order-independent digest, mutating the documents being compared.
	// The differ then emitted operations with indexes valid for the shuffled arrays
	// only, producing patches that randomly failed to apply because the hasher seed
	// changes on every run. See issue #61.
	src := []byte(`[["a","b","c"],0]`)
	tgt := []byte(`[["a","b","d"],1]`)

	for _, tc := range []struct {
		name string
		opts []Option
	}{
		{name: "equivalent", opts: []Option{Equivalent()}},
		{name: "equivalent+rationalize", opts: []Option{Equivalent(), Rationalize()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 200 {
				patch, err := CompareJSON(src, tgt, tc.opts...)
				if err != nil {
					t.Fatal(err)
				}
				got, err := patch.apply(src, true)
				if err != nil {
					t.Fatalf("failed to apply patch: %v", err)
				}
				if !bytes.Equal(got, tgt) {
					t.Fatalf("patch produced %s instead of %s: %s", got, tgt, patch)
				}
			}
		})
	}
}

func TestCompareJSONEquivalentUseNumber(t *testing.T) {
	unmarshalUseNumber := func(b []byte, v any) error {
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		return dec.Decode(v)
	}
	src := []byte(`{"a":[{"id":1,"n":0.80}]}`)
	tgt := []byte(`{"a":[{"id":1,"n":0.62}]}`)

	patch, err := CompareJSON(src, tgt,
		UnmarshalFunc(unmarshalUseNumber),
		Equivalent(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(patch) == 0 {
		t.Error("expected a patch for an array element whose number changed, got none")
	}
	t.Run("a pure reordering is still equivalent", func(t *testing.T) {
		src := []byte(`{"a":[{"id":1},{"id":2}]}`)
		tgt := []byte(`{"a":[{"id":2},{"id":1}]}`)

		patch, err := CompareJSON(src, tgt,
			UnmarshalFunc(unmarshalUseNumber),
			Equivalent(),
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(patch) != 0 {
			t.Errorf("expected an empty patch for a pure reordering, got %v", patch)
		}
	})
}

// Factorize must only turn an add into a copy/move when the source value
// really equals the added value; the internal apply helper copies op.Value
// and cannot see this, so check the emitted patch.
func TestCompareJSON_factorizeHashCollisions(t *testing.T) {
	for _, tc := range []struct{ src, tgt, want string }{
		{`{"a":[]}`, `{"a":[],"b":[""]}`, `{"value":[""],"op":"add","path":"/b"}`},
		{`{"a":"0"}`, `{"a":"0","b":false}`, `{"value":false,"op":"add","path":"/b"}`},
		{`{"a":{"x":"y"}}`, `{"a":{"x":"y"},"b":{"xy":""}}`, `{"value":{"xy":""},"op":"add","path":"/b"}`},
		{`{"a":[[1]]}`, `{"a":[[1]],"b":[1]}`, `{"value":[1],"op":"add","path":"/b"}`},
	} {
		patch, err := CompareJSON([]byte(tc.src), []byte(tc.tgt), Factorize())
		if err != nil {
			t.Fatal(err)
		}
		if got := patch.String(); got != tc.want {
			t.Errorf("%s -> %s: got %s, want %s", tc.src, tc.tgt, got, tc.want)
		}
	}
}

func TestCompareJSON_equivalentHashCollisions(t *testing.T) {
	for _, tc := range []struct{ src, tgt string }{
		{`[[1]]`, `[1]`},
		{`[["a","b"]]`, `[["ab"]]`},
		{`{"a":[""]}`, `{"a":[[]]}`},
	} {
		patch, err := CompareJSON([]byte(tc.src), []byte(tc.tgt), Equivalent())
		if err != nil {
			t.Fatal(err)
		}
		b, err := patch.apply([]byte(tc.src), false)
		if err != nil {
			t.Fatalf("%s -> %s: apply %s: %v", tc.src, tc.tgt, patch, err)
		}
		var got, want interface{}
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(tc.tgt), &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s -> %s: patch %s gives %s", tc.src, tc.tgt, patch, b)
		}
	}
}

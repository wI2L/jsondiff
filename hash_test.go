package jsondiff

import (
	"encoding/json"
	"hash/maphash"
	"os"
	"testing"
)

func readJSON(filename string) (any, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var i any
	if err := json.Unmarshal(b, &i); err != nil {
		return nil, err
	}
	return i, nil
}

func Test_digestValue(t *testing.T) {
	data, err := readJSON("testdata/examples/twitter.json")
	if err != nil {
		t.Error(err)
	}
	h := hasher{}

	n1 := h.digest(data, false)
	n2 := h.digest(data, false)

	if n1 != n2 {
		t.Errorf("expected hash sums to be equal: %d != %d", n1, n2)
	}
}

func Test_digestValue_jsonNumber(t *testing.T) {
	h := hasher{}

	t.Run("distinct numbers hash differently", func(t *testing.T) {
		a := map[string]any{"n": json.Number("0.80")}
		b := map[string]any{"n": json.Number("0.62")}

		if h.digest(a, false) == h.digest(b, false) {
			t.Error("expected different hash sums for different json.Number values")
		}
	})
	t.Run("equal numbers hash identically", func(t *testing.T) {
		a := map[string]any{"n": json.Number("0.80")}
		b := map[string]any{"n": json.Number("0.80")}

		if h.digest(a, false) != h.digest(b, false) {
			t.Error("expected equal hash sums for equal json.Number values")
		}
	})
	t.Run("integers beyond float64 precision hash differently", func(t *testing.T) {
		a := map[string]any{"id": json.Number("9007199254740992")}
		b := map[string]any{"id": json.Number("9007199254740993")}

		if h.digest(a, false) == h.digest(b, false) {
			t.Error("expected different hash sums for integers that share a float64 representation")
		}
	})
	t.Run("json.Number text does not collide with other scalars", func(t *testing.T) {
		pairs := [][2]any{
			{json.Number("0"), false},
			{json.Number("0"), nil},
		}
		for _, p := range pairs {
			a := map[string]any{"n": p[0]}
			b := map[string]any{"n": p[1]}

			if h.digest(a, false) == h.digest(b, false) {
				t.Errorf("expected different hash sums for %v and %v", p[0], p[1])
			}
		}
	})
}

func BenchmarkHashing(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	data, err := readJSON("testdata/examples/twitter.json")
	if err != nil {
		b.Error(err)
	}
	b.Run("hasher-digest", func(b *testing.B) {
		h := hasher{}
		for b.Loop() {
			_ = h.digest(data, false)
		}
	})
	b.Run("json.Marshal+hash", func(b *testing.B) {
		for b.Loop() {
			bts, err := json.Marshal(data)
			if err != nil {
				b.Error(err)
			}
			h := maphash.Hash{}
			if _, err := h.Write(bts); err != nil {
				b.Error(err)
			}
			_ = h.Sum64()
		}
	})
}

func Test_digestValue_distinctValues(t *testing.T) {
	pairs := [][2]string{
		{
			`""`,
			`[]`,
		},
		{
			`[""]`,
			`[]`,
		},
		{
			`"0"`,
			`false`,
		},
		{
			`"0"`,
			`null`,
		},
		{
			`false`,
			`null`,
		},
		{
			`"1"`,
			`true`,
		},
		{
			`1`,
			`[1]`,
		},
		{
			`[[1]]`,
			`[1]`,
		},
		{
			`[]`,
			`{}`,
		},
		{
			`["a","b"]`,
			`["ab"]`,
		},
		{
			`{"x":"y"}`,
			`{"xy":""}`,
		},
		{
			`{"a":[]}`,
			`{"a":{}}`,
		},
		{
			`[[],[1]]`,
			`[[1],[]]`,
		},
	}
	h := hasher{}
	for _, p := range pairs {
		var a, b any
		if err := json.Unmarshal([]byte(p[0]), &a); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(p[1]), &b); err != nil {
			t.Fatal(err)
		}
		if h.digest(a, false) == h.digest(b, false) {
			t.Errorf("%s and %s share the same digest", p[0], p[1])
		}
	}
}

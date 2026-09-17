package jsondiff

import (
	"bytes"
	"encoding/json"
	"hash/maphash"
	"os"
	"testing"
)

func readJSON(filename string) (interface{}, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var i interface{}
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

// Test_digestValue_jsonNumber covers values decoded with json.Decoder.UseNumber,
// which the UnmarshalFunc option exists to enable (see ExampleUnmarshalFunc).
// Without a json.Number case, the hasher's type switch falls through and the
// value contributes nothing to the digest, so two values differing only in their
// numbers hash identically.
func Test_digestValue_jsonNumber(t *testing.T) {
	h := hasher{}

	t.Run("distinct numbers hash differently", func(t *testing.T) {
		a := map[string]interface{}{"n": json.Number("0.80")}
		b := map[string]interface{}{"n": json.Number("0.62")}

		if h.digest(a, false) == h.digest(b, false) {
			t.Error("expected different hash sums for different json.Number values")
		}
	})

	t.Run("equal numbers hash identically", func(t *testing.T) {
		a := map[string]interface{}{"n": json.Number("0.80")}
		b := map[string]interface{}{"n": json.Number("0.80")}

		if h.digest(a, false) != h.digest(b, false) {
			t.Error("expected equal hash sums for equal json.Number values")
		}
	})

	t.Run("integers beyond float64 precision hash differently", func(t *testing.T) {
		// 9007199254740992 is 2^53. As float64, 2^53 and 2^53+1 are the same
		// value, so hashing via Float64() would collapse these two into one
		// digest even though the differ compares them as different.
		a := map[string]interface{}{"id": json.Number("9007199254740992")}
		b := map[string]interface{}{"id": json.Number("9007199254740993")}

		if h.digest(a, false) == h.digest(b, false) {
			t.Error("expected different hash sums for integers that share a float64 representation")
		}
	})
}

// TestCompareJSONEquivalentUseNumber is the end-to-end case the hasher gap
// produced: with Equivalent enabled, an array whose elements differ only in
// numeric fields was reported as unchanged.
func TestCompareJSONEquivalentUseNumber(t *testing.T) {
	unmarshalUseNumber := func(b []byte, v any) error {
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		return dec.Decode(v)
	}
	source := []byte(`{"a":[{"id":1,"n":0.80}]}`)
	target := []byte(`{"a":[{"id":1,"n":0.62}]}`)

	patch, err := CompareJSON(source, target,
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

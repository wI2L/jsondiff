package jsondiff

import (
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

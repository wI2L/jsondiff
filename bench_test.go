package jsondiff

import (
	"encoding/json"
	"os"
	"testing"
)

func BenchmarkMedium(b *testing.B) {
	beforeBytes, err := os.ReadFile("testdata/benchs/medium/before.json")
	if err != nil {
		b.Fatal(err)
	}
	afterBytesOrdered, err := os.ReadFile("testdata/benchs/medium/after-ordered.json")
	if err != nil {
		b.Fatal(err)
	}
	afterBytesUnordered, err := os.ReadFile("testdata/benchs/medium/after-unordered.json")
	if err != nil {
		b.Fatal(err)
	}
	makeopts := func(opts ...Option) []Option { return opts }

	for _, bb := range []struct {
		name       string
		opts       []Option
		afterBytes []byte
	}{
		{"default-ordered", nil, afterBytesOrdered},
		{"default-unordered", nil, afterBytesUnordered},
		{"invertible", makeopts(Invertible()), afterBytesOrdered},
		{"factorize", makeopts(Factorize()), afterBytesOrdered},
		{"rationalize", makeopts(Rationalize()), afterBytesOrdered},
		{"equivalent-ordered", makeopts(Equivalent()), afterBytesOrdered},
		{"equivalent-unordered", makeopts(Equivalent()), afterBytesUnordered},
		{"factor+ratio", makeopts(Factorize(), Rationalize()), afterBytesOrdered},
		{"all-ordered", makeopts(Factorize(), Rationalize(), Invertible(), Equivalent()), afterBytesOrdered},
		{"all-unordered", makeopts(Factorize(), Rationalize(), Invertible(), Equivalent()), afterBytesUnordered},
	} {
		var before, after interface{}
		err = json.Unmarshal(beforeBytes, &before)
		if err != nil {
			b.Fatal(err)
		}
		err = json.Unmarshal(bb.afterBytes, &after)
		if err != nil {
			b.Fatal(err)
		}
		b.Run("Compare/"+bb.name, func(b *testing.B) {
			if testing.Short() {
				b.Skip()
			}
			for i := 0; i < b.N; i++ {
				patch, err := CompareOpts(before, after, bb.opts...)
				if err != nil {
					b.Error(err)
				}
				_ = patch
			}
		})
		b.Run("CompareJSON/"+bb.name, func(b *testing.B) {
			if testing.Short() {
				b.Skip()
			}
			for i := 0; i < b.N; i++ {
				patch, err := CompareJSONOpts(beforeBytes, bb.afterBytes, bb.opts...)
				if err != nil {
					b.Error(err)
				}
				_ = patch
			}
		})
		b.Run("DifferCompare/"+bb.name, func(b *testing.B) {
			if testing.Short() {
				b.Skip()
			}
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				d := Differ{targetBytes: bb.afterBytes}
				for _, opt := range bb.opts {
					opt(&d)
				}
				d.Compare(before, after)
			}
		})
		b.Run("DifferResetCompare/"+bb.name, func(b *testing.B) {
			d := Differ{targetBytes: bb.afterBytes}
			for _, opt := range bb.opts {
				opt(&d)
			}
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				d.Compare(before, after)
				d.Reset()
			}
		})
	}
}

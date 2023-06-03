package jsondiff

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const benchsDir = "testdata/benchs"

func BenchmarkSmall(b *testing.B)  { benchmark(b, "small") }
func BenchmarkMedium(b *testing.B) { benchmark(b, "medium") }

func benchmark(b *testing.B, dir string) {
	src, err := os.ReadFile(filepath.Join(benchsDir, dir, "source.json"))
	if err != nil {
		b.Fatal(err)
	}
	tgt, err := os.ReadFile(filepath.Join(benchsDir, dir, "target.json"))
	if err != nil {
		b.Fatal(err)
	}
	tgtUnordered, err := os.ReadFile(filepath.Join(benchsDir, dir, "target.unordered.json"))
	if err != nil {
		b.Fatal(err)
	}
	subBenchmarks(b, src, tgt, tgtUnordered)
}

func subBenchmarks(b *testing.B, src, tgt, tgtUnordered []byte) {
	makeopts := func(opts ...Option) []Option { return opts }

	for _, bb := range []struct {
		name       string
		opts       []Option
		afterBytes []byte
	}{
		{"default", nil, tgt},
		{"default-unordered", nil, tgtUnordered},
		{"invertible", makeopts(Invertible()), tgt},
		{"factorize", makeopts(Factorize()), tgt},
		{"rationalize", makeopts(Rationalize()), tgt},
		{"equivalent", makeopts(Equivalent()), tgt},
		{"equivalent-unordered", makeopts(Equivalent()), tgtUnordered},
		{"factor+ratio", makeopts(Factorize(), Rationalize()), tgt},
		{"all", makeopts(Factorize(), Rationalize(), Invertible(), Equivalent()), tgt},
		{"all-unordered", makeopts(Factorize(), Rationalize(), Invertible(), Equivalent()), tgtUnordered},
	} {
		var before, after interface{}

		if err := json.Unmarshal(src, &before); err != nil {
			b.Fatal(err)
		}
		if err := json.Unmarshal(bb.afterBytes, &after); err != nil {
			b.Fatal(err)
		}
		b.Run("DifferReset/"+bb.name, func(b *testing.B) {
			d := Differ{
				targetBytes: compactBytes(bb.afterBytes),
				isCompact:   true,
			}
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
		b.Run("Differ/"+bb.name, func(b *testing.B) {
			targetBytes := compactBytes(bb.afterBytes)
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				d := Differ{
					targetBytes: targetBytes,
					isCompact:   true,
				}
				for _, opt := range bb.opts {
					opt(&d)
				}
				d.Compare(before, after)
			}
		})
		b.Run("CompareJSON/"+bb.name, func(b *testing.B) {
			if testing.Short() {
				b.Skip()
			}
			for i := 0; i < b.N; i++ {
				patch, err := CompareJSONOpts(src, bb.afterBytes, bb.opts...)
				if err != nil {
					b.Error(err)
				}
				_ = patch
			}
		})
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
	}
}

func compactBytes(src []byte) []byte {
	b := make([]byte, 0, len(src))
	copy(b, src)
	b = _compact(b, b)
	return b
}

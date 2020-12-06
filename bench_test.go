package jsondiff

import (
	"io/ioutil"
	"testing"
)

func BenchmarkCompareJSONOpts(b *testing.B) {
	beforeBytes, err := ioutil.ReadFile("testdata/benchs/before.json")
	if err != nil {
		b.Fatal(err)
	}
	afterBytes, err := ioutil.ReadFile("testdata/benchs/after.json")
	if err != nil {
		b.Fatal(err)
	}
	makeopts := func(opts ...Option) []Option { return opts }

	for _, bb := range []struct {
		name string
		opts []Option
	}{
		{"default", nil},
		{"invertible", makeopts(Invertible())},
		{"factorize", makeopts(Factorize())},
		{"rationalize", makeopts(Rationalize())},
		{"factor+ratio", makeopts(Factorize(), Rationalize())},
		{"all-options", makeopts(Factorize(), Rationalize(), Invertible())},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				patch, err := CompareJSONOpts(beforeBytes, afterBytes, bb.opts...)
				if err != nil {
					b.Error(err)
				}
				_ = patch
			}
		})
	}
}

package jsondiff

import (
	"encoding/json"
	"io/ioutil"
	"testing"
)

func BenchmarkCompare(b *testing.B) {
	beforeBytes, err := ioutil.ReadFile("testdata/benchs/before.json")
	if err != nil {
		b.Fatal(err)
	}
	var before interface{}
	err = json.Unmarshal(beforeBytes, &before)
	if err != nil {
		b.Fatal(err)
	}
	afterBytes, err := ioutil.ReadFile("testdata/benchs/after.json")
	if err != nil {
		b.Fatal(err)
	}
	var after interface{}
	err = json.Unmarshal(afterBytes, &after)
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
		b.Run("Compare/"+bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				patch, err := CompareOpts(before, after, bb.opts...)
				if err != nil {
					b.Error(err)
				}
				_ = patch
			}
		})
		b.Run("CompareJSON/"+bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				patch, err := CompareJSONOpts(beforeBytes, afterBytes, bb.opts...)
				if err != nil {
					b.Error(err)
				}
				_ = patch
			}
		})
		b.Run("differ_diff/"+bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				d := differ{
					targetBytes: afterBytes,
				}
				for _, opt := range bb.opts {
					opt(&d)
				}
				d.diff(before, after)
			}
		})
	}
}

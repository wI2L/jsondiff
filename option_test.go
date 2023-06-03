package jsondiff

import (
	"fmt"
	"testing"
)

func TestDiffer_applyOpts(t *testing.T) {
	d := Differ{}

	var (
		marshal      = func(any) ([]byte, error) { return nil, nil }
		unmarshal    = func([]byte, any) error { return nil }
		ignoredPaths = []string{
			"/a/b/c",
			"/x/0/y/2/z/3",
		}
	)
	d.applyOpts(
		Factorize(),
		Rationalize(),
		Equivalent(),
		Invertible(),
		MarshalFunc(marshal),
		UnmarshalFunc(unmarshal),
		SkipCompact(),
		InPlaceCompaction(),
		Ignores(ignoredPaths...),
	)
	if d.opts.factorize != true {
		t.Errorf("factorize option is not enabled")
	}
	if d.opts.rationalize != true {
		t.Errorf("rationalize option is not enabled")
	}
	if d.opts.equivalent != true {
		t.Errorf("equivalent option is not enabled")
	}
	if d.opts.invertible != true {
		t.Errorf("invertible option is not enabled")
	}
	if d.isCompact != true {
		t.Errorf("input not marked as compact")
	}
	if d.compactInPlace != true {
		t.Errorf("in-place compaction disabled")
	}
	if !cmpFuncs(d.opts.marshal, marshal) {
		t.Errorf("marshal funcs mismatch")
	}
	if !cmpFuncs(d.opts.unmarshal, unmarshal) {
		t.Errorf("unmarshal funcs mismatch")
	}
	if d.opts.hasIgnore != true {
		t.Errorf("differ has no ignored paths")
	} else {
		if len(d.opts.ignores) != len(ignoredPaths) {
			t.Errorf("ignored paths map length mismatch input")
		}
	}
}

func cmpFuncs(x, y any) bool {
	// Hacky comparison of the function addresses
	// since the spec does not allow to compare funcs:
	// https://go.dev/ref/spec#Comparison_operators
	return fmt.Sprintf("%p", x) == fmt.Sprintf("%p", y)
}

package jsondiff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var testNameReplacer = strings.NewReplacer(",", "", "(", "", ")", "")

type testcase struct {
	Name            string      `json:"name"`
	Before          interface{} `json:"before"`
	After           interface{} `json:"after"`
	Patch           Patch       `json:"patch"`
	IncompletePatch Patch       `json:"incomplete_patch"`
	Ignores         []string    `json:"ignores"`
}

type patchGetter func(tc *testcase) Patch

func TestArrayCases(t *testing.T)  { runCasesFromFile(t, "testdata/tests/array.json") }
func TestObjectCases(t *testing.T) { runCasesFromFile(t, "testdata/tests/object.json") }
func TestRootCases(t *testing.T)   { runCasesFromFile(t, "testdata/tests/root.json") }

func TestDiffer_Reset(t *testing.T) {
	d := &Differ{
		ptr: pointer{
			buf: make([]byte, 15, 15),
			end: 15,
		},
		hashmap: map[uint64]jsonNode{
			1: {},
		},
		patch: make([]Operation, 42, 42),
	}
	d.Reset()

	if l := len(d.patch); l != 0 {
		t.Errorf("expected empty patch collection, got length %d", l)
	}
	if l := len(d.hashmap); l != 0 {
		t.Errorf("expected cleared hashmap, got length %d", l)
	}
	if d.ptr.end != 0 {
		t.Errorf("expected reset ptr")
	}
	if l := len(d.ptr.buf); l != 0 {
		t.Errorf("expected empty ptr buf, got length %d", l)
	}
}

func TestOptions(t *testing.T) {
	makeopts := func(opts ...Option) []Option { return opts }

	for _, tt := range []struct {
		testfile string
		options  []Option
	}{
		{"testdata/tests/options/invertible.json", makeopts(Invertible())},
		{"testdata/tests/options/factorization.json", makeopts(Factorize())},
		{"testdata/tests/options/rationalization.json", makeopts(Rationalize())},
		{"testdata/tests/options/equivalence.json", makeopts(Equivalent())},
		{"testdata/tests/options/ignore.json", makeopts()},
		{"testdata/tests/options/all.json", makeopts(Factorize(), Rationalize(), Invertible(), Equivalent())},
	} {
		var (
			ext  = filepath.Ext(tt.testfile)
			base = filepath.Base(tt.testfile)
			name = strings.TrimSuffix(base, ext)
		)
		t.Run(name, func(t *testing.T) {
			runCasesFromFile(t, tt.testfile, tt.options...)
		})
	}
}

func runCasesFromFile(t *testing.T, filename string, opts ...Option) {
	b, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var cases []testcase
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	runTestCases(t, cases, opts...)
}

func runTestCases(t *testing.T, cases []testcase, opts ...Option) {
	for _, tc := range cases {
		name := testNameReplacer.Replace(tc.Name)

		t.Run(name, func(t *testing.T) {
			runTestCase(t, tc, func(tc *testcase) Patch {
				return tc.Patch
			}, opts...)
		})
		if len(tc.Ignores) != 0 {
			name = fmt.Sprintf("%s_with_ignore", name)
			xopts := append(opts, Ignores(tc.Ignores...))

			t.Run(name, func(t *testing.T) {
				runTestCase(t, tc, func(tc *testcase) Patch {
					return tc.IncompletePatch
				}, xopts...)
			})
		}
	}
}

func runTestCase(t *testing.T, tc testcase, pc patchGetter, opts ...Option) {
	beforeBytes, err := json.Marshal(tc.Before)
	if err != nil {
		t.Error(err)
	}
	d := &Differ{
		targetBytes: beforeBytes,
	}
	d = d.WithOpts(opts...)
	d.Compare(tc.Before, tc.After)

	patch := d.Patch()
	wantPatch := pc(&tc)

	if patch != nil {
		t.Logf("\n%s", patch)
	}
	if len(patch) != len(wantPatch) {
		t.Errorf("got %d patches, want %d", len(patch), len(wantPatch))
		return
	}
	for i, op := range patch {
		want := wantPatch[i]
		if g, w := op.Type, want.Type; g != w {
			t.Errorf("op #%d mismatch: op: got %q, want %q", i, g, w)
		}
		if g, w := op.Path, want.Path; g != w {
			t.Errorf("op #%d mismatch: path: got %q, want %q", i, g, w)
		}
		switch want.Type {
		case OperationCopy, OperationMove:
			if g, w := op.From, want.From; g != w {
				t.Errorf("op #%d mismatch: from: got %q, want %q", i, g, w)
			}
		case OperationAdd, OperationReplace:
			if !reflect.DeepEqual(op.Value, want.Value) {
				t.Errorf("op #%d mismatch: value: unequal", i)
			}
		}
	}
}

func Benchmark_sortStrings(b *testing.B) {
	for _, v := range [][]string{
		// 5
		{
			"medieval",
			"bike",
			"trust",
			"sodium",
			"hemisphere",
		},
		// 10
		{
			"general",
			"lamp",
			"journal",
			"common",
			"grind",
			"hay",
			"dismiss",
			"sunrise",
			"shoulder",
			"certain",
		},
		// 15
		{
			"plant",
			"instinct",
			"infect",
			"transaction",
			"transport",
			"beer",
			"printer",
			"neutral",
			"collect",
			"message",
			"chaos",
			"dynamic",
			"justice",
			"master",
			"want",
		},
		// 20
		{
			"absorption",
			"ditch",
			"gradual",
			"leftovers",
			"lace",
			"clash",
			"fun",
			"stereotype",
			"lamp",
			"deter",
			"circle",
			"lay",
			"murder",
			"grimace",
			"jacket",
			"have",
			"ambiguous",
			"pit",
			"plug",
			"notice",
		},
		// 25
		{
			"flesh",
			"kidney",
			"hard",
			"carbon",
			"ignorant",
			"pocket",
			"strategic",
			"allow",
			"advance",
			"impulse",
			"infinite",
			"integrated",
			"expenditure",
			"technology",
			"prevent",
			"valid",
			"revive",
			"manager",
			"sheep",
			"kitchen",
			"guest",
			"dismissal",
			"divide",
			"bow",
			"buffet",
		},
	} {
		b.Run(fmt.Sprintf("sort.Strings-%d", len(v)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				sort.Strings(v)
			}
		})
		b.Run(fmt.Sprintf("sortStrings-%d", len(v)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				sortStrings(v)
			}
		})
	}
}

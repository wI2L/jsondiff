package jsondiff

import (
	"bytes"
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
	Name          string      `json:"name"`
	Before        interface{} `json:"before"`
	After         interface{} `json:"after"`
	Patch         Patch       `json:"patch"`
	PartialPatch  Patch       `json:"partial_patch"`
	Ignores       []string    `json:"ignores"`
	SkipApplyTest bool        `json:"skip_apply_test"`
}

type patchGetter func(tc *testcase) Patch

func TestArrayCases(t *testing.T)  { runCasesFromFile(t, "testdata/tests/array.json") }
func TestObjectCases(t *testing.T) { runCasesFromFile(t, "testdata/tests/object.json") }
func TestRootCases(t *testing.T)   { runCasesFromFile(t, "testdata/tests/root.json") }

func TestDiffer_Reset(t *testing.T) {
	d := &Differ{
		ptr: pointer{
			buf: make([]byte, 15),
			sep: 15,
		},
		hashmap: map[uint64]jsonNode{
			1: {},
		},
		patch: make([]Operation, 42),
	}
	d.Reset()

	if l := len(d.patch); l != 0 {
		t.Errorf("expected empty patch collection, got length %d", l)
	}
	if l := len(d.hashmap); l != 0 {
		t.Errorf("expected cleared hashmap, got length %d", l)
	}
	if d.ptr.sep != 0 {
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
		{"testdata/tests/options/lcs.json", makeopts(LCS())},
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
	t.Helper()

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
	t.Helper()

	for _, tc := range cases {
		name := testNameReplacer.Replace(tc.Name)

		t.Run(name, func(t *testing.T) {
			runTestCase(t, tc, func(tc *testcase) Patch {
				return tc.Patch
			}, opts...)
		})
		if tc.Ignores != nil {
			name = fmt.Sprintf("%s_with_ignore", name)
			xopts := append(opts, Ignores(tc.Ignores...)) //nolint:gocritic

			t.Run(name, func(t *testing.T) {
				runTestCase(t, tc, func(tc *testcase) Patch {
					return tc.PartialPatch
				}, xopts...)
			})
		}
	}
}

func runTestCase(t *testing.T, tc testcase, pc patchGetter, opts ...Option) {
	t.Helper()

	afterBytes, err := json.Marshal(tc.After)
	if err != nil {
		t.Error(err)
	}
	d := &Differ{
		targetBytes: afterBytes,
	}
	d = d.WithOpts(opts...)
	d.Compare(tc.Before, tc.After)

	patch, wantPatch := d.Patch(), pc(&tc)

	if patch != nil {
		t.Logf("\n%s", patch)
	}
	if len(patch) != len(wantPatch) {
		t.Errorf("got %d operations, want %d", len(patch), len(wantPatch))
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
	// Unsupported cases:
	//  * the Ignores() option is enabled
	//  * explicitly disabled for individual test case
	if d.opts.ignores != nil || tc.SkipApplyTest {
		return
	}
	mustMarshal := func(v any) []byte {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Errorf("marshaling error: %s", err)
		}
		return b
	}
	// Validate that the patch is fundamentally correct by
	// applying it to the source document, and compare the
	// result with the expected document.
	b, err := patch.apply(mustMarshal(tc.Before), false)
	if err != nil {
		t.Errorf("failed to apply patch: %s", err)
	}
	if !bytes.Equal(b, mustMarshal(tc.After)) {
		t.Errorf("patch does not produce the expected changes")
		t.Logf("got: %s", string(b))
		t.Logf("want: %s", string(mustMarshal(tc.After)))
	}
}

func TestDiffer_unorderedDeepEqualSlice(t *testing.T) {
	for _, tt := range []struct {
		src, tgt []interface{}
		equal    bool
	}{
		{
			src:   []interface{}{1, 2, 3},
			tgt:   []interface{}{3, 2, 1},
			equal: true,
		},
		{
			src:   []interface{}{1, 2, 3},
			tgt:   []interface{}{4, 3, 2, 1},
			equal: false,
		},
		{
			src: []interface{}{
				"foo",
				map[string]interface{}{"A": "AAA"},
				map[string]interface{}{"B": "BBB"},
				"foo",
				"bar",
			},
			tgt: []interface{}{
				"foo",
				"foo",
				map[string]interface{}{"A": "AAA"},
				map[string]interface{}{"B": "BBB"},
				"bar",
			},
			equal: true,
		},
	} {
		d := Differ{}
		eq := d.unorderedDeepEqualSlice(tt.src, tt.tgt)
		if eq != tt.equal {
			t.Errorf("equality mismatch, got %t, want %t", eq, tt.equal)
		}
	}
}

func Test_issue17(t *testing.T) {
	type (
		VolumeMount struct {
			Name      string `json:"name"`
			MountPath string `json:"mountPath"`
		}
		Container struct {
			VolumeMounts []VolumeMount `json:"volumeMounts,omitempty"`
		}
	)
	src := Container{
		VolumeMounts: []VolumeMount{{
			Name:      "name1",
			MountPath: "/foo/bar/1",
		}, {
			Name:      "name2",
			MountPath: "/foo/bar/2",
		}, {
			Name:      "name3",
			MountPath: "/foo/bar/3",
		}, {
			Name:      "name4",
			MountPath: "/foo/bar/4",
		}, {
			Name:      "name5",
			MountPath: "/foo/bar/5",
		}, {
			Name:      "name6",
			MountPath: "/foo/bar/6",
		}},
	}
	tgt := Container{
		VolumeMounts: []VolumeMount{{
			Name:      "name1",
			MountPath: "/foo/bar/1",
		}, {
			Name:      "name2",
			MountPath: "/foo/bar/2",
		}, {
			Name:      "name4",
			MountPath: "/foo/bar/4",
		}, {
			Name:      "name5",
			MountPath: "/foo/bar/5",
		}, {
			Name:      "name6",
			MountPath: "/foo/bar/6",
		}},
	}
	patch, _ := Compare(src, tgt, LCS())

	if len(patch) != 1 {
		t.Errorf("expected a patch with 1 operation, got %d", len(patch))
	}
	b, _ := json.Marshal(patch)
	t.Logf("%s", string(b))
}

func Benchmark_sortStrings(b *testing.B) {
	if testing.Short() {
		b.Skip()
	}
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

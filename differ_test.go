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

func TestRFCCases(t *testing.T) {
	runCasesFromFile(t, "testdata/tests/jsonpatch/rfc.json", Factorize(), LCS())
}                                  // https://datatracker.ietf.org/doc/html/rfc6902#appendix-A
func TestArrayCases(t *testing.T)  { runCasesFromFile(t, "testdata/tests/jsonpatch/array.json") }
func TestObjectCases(t *testing.T) { runCasesFromFile(t, "testdata/tests/jsonpatch/object.json") }
func TestRootCases(t *testing.T)   { runCasesFromFile(t, "testdata/tests/jsonpatch/root.json") }

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
	makeOpts := func(opts ...Option) []Option { return opts }

	for _, tc := range []struct {
		testFile string
		options  []Option
	}{
		{"testdata/tests/jsonpatch/options/invertible.json", makeOpts(Invertible())},
		{"testdata/tests/jsonpatch/options/factorization.json", makeOpts(Factorize())},
		{"testdata/tests/jsonpatch/options/rationalization.json", makeOpts(Rationalize())},
		{"testdata/tests/jsonpatch/options/equivalence.json", makeOpts(Equivalent())},
		{"testdata/tests/jsonpatch/options/ignore.json", makeOpts()},
		{"testdata/tests/jsonpatch/options/lcs.json", makeOpts(LCS(), Factorize())},
		{"testdata/tests/jsonpatch/options/all.json", makeOpts(Factorize(), Rationalize(), Invertible(), Equivalent())},
		{"testdata/tests/jsonpatch/options/lcs+equivalence.json", makeOpts(LCS(), Equivalent())},
	} {
		name := strings.TrimSuffix(filepath.Base(tc.testFile), filepath.Ext(tc.testFile))
		t.Run(name, func(t *testing.T) {
			runCasesFromFile(t, tc.testFile, tc.options...)
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
			extendedOpts := append(opts, Ignores(tc.Ignores...)) //nolint:gocritic

			t.Run(name, func(t *testing.T) {
				runTestCase(t, tc, func(tc *testcase) Patch {
					return tc.PartialPatch
				}, extendedOpts...)
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
	// Re-marshal the patched document to ensure it follows
	// the Golang JSON convention of ordering map keys, and
	// can be compared to the target document.
	before, after := unmarshalMarshal(t, b), mustMarshal(tc.After)

	if !bytes.Equal(before, after) {
		t.Errorf("patch does not produce the expected changes")
		t.Logf("got: %s", string(before))
		t.Logf("want: %s", string(after))
	}
}

func TestDiffer_unorderedDeepEqualSlice(t *testing.T) {
	for _, tc := range []struct {
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
		eq := d.unorderedDeepEqualSlice(tc.src, tc.tgt)
		if eq != tc.equal {
			t.Errorf("equality mismatch, got %t, want %t", eq, tc.equal)
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

func Test_issue29(t *testing.T) {
	src := []byte(`{"a":{"b":[{"c":[4,5]},2,1]}}`)
	tgt := []byte(`{"a":{"b":[{"c":[5,4]},1,2]}}`)

	patch, err := CompareJSON(src, tgt, Equivalent())
	if err != nil {
		t.Error(err)
	}
	if len(patch) != 0 {
		t.Errorf("expected 0 operations, got %d", len(patch))
	}
	t.Log(patch)
}

func Test_issue29_alt(t *testing.T) {
	src := []byte(`{"a":{"b":[[7,6],2,[42,84]]}}`)
	tgt := []byte(`{"a":{"b":[[6,7],1,[84,42]]}}`)

	patch, err := CompareJSON(src, tgt, Equivalent())
	if err != nil {
		t.Error(err)
	}
	if len(patch) != 1 {
		t.Errorf("expected 1 operations, got %d", len(patch))
		t.Log(patch)
	}
	if op := patch[0]; op.Path != "/a/b/1" && op.Type != OperationReplace {
		t.Errorf("expected replace operation at path /a/b/1, got %s at %s", op.Type, op.Path)
	}
}

func Benchmark_sortStrings(b *testing.B) {
	if testing.Short() {
		b.Skip()
	}
	for _, v := range [][]string{
		{ // 5
			"medieval",
			"bike",
			"trust",
			"sodium",
			"hemisphere",
		},
		{ // 10
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
		{ // 15
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
		{ // 20
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
		{ // 25
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
			for b.Loop() {
				sort.Strings(v)
			}
		})
		b.Run(fmt.Sprintf("sortStrings-%d", len(v)), func(b *testing.B) {
			for b.Loop() {
				sortStrings(v)
			}
		})
	}
}

func unmarshalMarshal(t *testing.T, b []byte) []byte {
	t.Helper()

	var i interface{}
	if err := json.Unmarshal(b, &i); err != nil {
		t.Error(err)
	}
	b2, err := json.Marshal(i)
	if err != nil {
		t.Error(err)
	}
	return b2
}

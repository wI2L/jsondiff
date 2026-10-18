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
	Name          string   `json:"name"`
	Before        any      `json:"before"`
	After         any      `json:"after"`
	Patch         Patch    `json:"patch"`
	PartialPatch  Patch    `json:"partial_patch"`
	Ignores       []string `json:"ignores"`
	SkipApplyTest bool     `json:"skip_apply_test"`
}

type patchGetter func(tc *testcase) Patch

func TestRFCCases(t *testing.T) {
	// https://datatracker.ietf.org/doc/html/rfc6902#appendix-A
	runCasesFromFile(t, "testdata/tests/jsonpatch/rfc.json", Factorize(), LCS())
}

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
		src, tgt []any
		equal    bool
	}{
		{
			src:   []any{1, 2, 3},
			tgt:   []any{3, 2, 1},
			equal: true,
		},
		{
			src:   []any{1, 2, 3},
			tgt:   []any{4, 3, 2, 1},
			equal: false,
		},
		{
			src: []any{
				"foo",
				map[string]any{"A": "AAA"},
				map[string]any{"B": "BBB"},
				"foo",
				"bar",
			},
			tgt: []any{
				"foo",
				"foo",
				map[string]any{"A": "AAA"},
				map[string]any{"B": "BBB"},
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

func Test_issue45(t *testing.T) {
	src := []byte(`{"key":{"b":{"$numberInt":"1"}},"name":"b1"}`)
	tgt := []byte(`{"key":{"bb":{"$numberInt":"1"}},"name":"b1"}`)

	patch, err := CompareJSON(src, tgt)
	if err != nil {
		t.Fatal(err)
	}
	if len(patch) != 2 {
		t.Fatalf("expected 2 operations without factorize, got %d", len(patch))
	}
	if op := patch[0]; op.Type != OperationRemove || op.Path != "/key/b" {
		t.Fatalf("expected first operation to remove /key/b, got %s at %s", op.Type, op.Path)
	}
	if op := patch[1]; op.Type != OperationAdd || op.Path != "/key/bb" {
		t.Fatalf("expected second operation to add /key/bb, got %s at %s", op.Type, op.Path)
	}

	patch, err = CompareJSON(src, tgt, Factorize())
	if err != nil {
		t.Fatal(err)
	}
	if len(patch) != 1 {
		t.Fatalf("expected 1 operation with factorize, got %d", len(patch))
	}
	if op := patch[0]; op.Type != OperationMove || op.From != "/key/b" || op.Path != "/key/bb" {
		t.Fatalf("expected move from /key/b to /key/bb, got %s from %s to %s", op.Type, op.From, op.Path)
	}
}

func Test_issue58(t *testing.T) {
	src := []byte(`[1,1,2]`)
	tgt := []byte(`[1,2,2]`)

	patch, err := CompareJSON(src, tgt, Equivalent())
	if err != nil {
		t.Fatal(err)
	}
	if len(patch) != 1 {
		t.Fatalf("expected 1 operation, got %d: %s", len(patch), patch)
	}
	if op := patch[0]; op.Type != OperationReplace || op.Path != "/1" {
		t.Errorf("expected replace operation at path /1, got %s at %s", op.Type, op.Path)
	}
}

func TestHasProperPathPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
		path   string
		want   bool
	}{
		// root pointer
		{
			name:   "root is a proper prefix of any non-root path",
			prefix: "",
			path:   "/a",
			want:   true,
		},
		{
			name:   "root is a proper prefix of a deep path",
			prefix: "",
			path:   "/a/b/c",
			want:   true,
		},
		{
			name:   "root is NOT a proper prefix of itself",
			prefix: "",
			path:   "",
			want:   false,
		},
		// exact match
		{
			name:   "identical paths are not a proper prefix",
			prefix: "/a",
			path:   "/a",
			want:   false,
		},
		{
			name:   "identical deep paths are not a proper prefix",
			prefix: "/a/b",
			path:   "/a/b",
			want:   false,
		},
		// genuine parent → child relationship
		{
			name:   "direct child is a proper prefix",
			prefix: "/a",
			path:   "/a/b",
			want:   true,
		},
		{
			name:   "grandchild is a proper prefix",
			prefix: "/a",
			path:   "/a/b/c",
			want:   true,
		},
		{
			name:   "nested direct child is a proper prefix",
			prefix: "/key/b",
			path:   "/key/b/child",
			want:   true,
		},
		// sibling paths that share a string prefix
		{
			name:   "sibling path /key/bb is NOT a proper prefix of /key/b",
			prefix: "/key/bb",
			path:   "/key/b",
			want:   false,
		},
		{
			name:   "sibling path /key/b is NOT a proper prefix of /key/bb",
			prefix: "/key/b",
			path:   "/key/bb",
			want:   false,
		},
		{
			name:   "sibling /a is NOT a proper prefix of /ab",
			prefix: "/a",
			path:   "/ab",
			want:   false,
		},
		{
			name:   "sibling /foo/bar is NOT a proper prefix of /foo/barz",
			prefix: "/foo/bar",
			path:   "/foo/barz",
			want:   false,
		},
		// unrelated paths
		{
			name:   "completely different paths",
			prefix: "/a",
			path:   "/b",
			want:   false,
		},
		{
			name:   "path is shorter than prefix",
			prefix: "/a/b/c",
			path:   "/a/b",
			want:   false,
		},
		// array index paths
		{
			name:   "array parent is a proper prefix of element",
			prefix: "/0",
			path:   "/0/name",
			want:   true,
		},
		{
			name:   "array index /1 is NOT a proper prefix of /10",
			prefix: "/1",
			path:   "/10",
			want:   false,
		},
		{
			name:   "array index /1 is NOT a proper prefix of /11",
			prefix: "/1",
			path:   "/11",
			want:   false,
		},
		{
			name:   "array index /10 is a proper prefix of /10/x",
			prefix: "/10",
			path:   "/10/x",
			want:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := hasProperPathPrefix(tc.prefix, tc.path)
			if got != tc.want {
				t.Errorf("hasProperPathPrefix(%q, %q) = %v, want %v",
					tc.prefix, tc.path, got, tc.want)
			}
		})
	}
}

func TestRationalizeIgnoredDescendant(t *testing.T) {
	for _, tc := range []struct {
		name    string
		before  string
		after   string
		ignored string
		opts    []Option
		want    string
		wantOps int
	}{
		// The ignored value differs between the two documents, the
		// replacement of an ancestor would clobber it.
		{
			name:    "object",
			before:  `{"keep":0,"a":0,"b":0,"c":0}`,
			after:   `{"keep":9,"a":1,"b":1,"c":1}`,
			ignored: "/keep",
			want:    `{"keep":0,"a":1,"b":1,"c":1}`,
		},
		{
			name:    "nested",
			before:  `{"a/b":{"keep":0,"a":0,"b":0,"c":0}}`,
			after:   `{"a/b":{"keep":9,"a":1,"b":1,"c":1}}`,
			ignored: "/a~1b/keep",
			want:    `{"a/b":{"keep":0,"a":1,"b":1,"c":1}}`,
		},
		{
			name:    "array",
			before:  `[0,0,0,0]`,
			after:   `[9,1,1,1]`,
			ignored: "/0",
			want:    `[0,1,1,1]`,
		},
		{
			name:    "null-vs-value",
			before:  `{"keep":null,"a":0}`,
			after:   `{"keep":5,"a":1}`,
			ignored: "/keep",
			want:    `{"keep":null,"a":1}`,
		},
		// The ignored value is identical in both documents, or absent
		// from both, so the operations can be rationalized.
		{
			name:    "unchanged",
			before:  `{"keep":0,"a":0,"b":0,"c":0}`,
			after:   `{"keep":0,"a":1,"b":1,"c":1}`,
			ignored: "/keep",
			want:    `{"keep":0,"a":1,"b":1,"c":1}`,
			wantOps: 1,
		},
		{
			name:    "nested unchanged",
			before:  `{"a/b":{"keep":0,"x":0}}`,
			after:   `{"a/b":{"keep":0,"x":1}}`,
			ignored: "/a~1b/keep",
			want:    `{"a/b":{"keep":0,"x":1}}`,
			wantOps: 1,
		},
		{
			name:    "array unchanged",
			before:  `[0,{"keep":0},2]`,
			after:   `[1,{"keep":0},3]`,
			ignored: "/1/keep",
			want:    `[1,{"keep":0},3]`,
			wantOps: 1,
		},
		{
			name:    "absent",
			before:  `{"a":0,"b":0}`,
			after:   `{"a":1,"b":1}`,
			ignored: "/ghost",
			want:    `{"a":1,"b":1}`,
			wantOps: 1,
		},
		{
			name:    "null equal",
			before:  `{"keep":null,"a":0}`,
			after:   `{"keep":null,"a":1}`,
			ignored: "/keep",
			want:    `{"keep":null,"a":1}`,
			wantOps: 1,
		},
		{
			name:    "numeric key",
			before:  `{"0":0,"a":0}`,
			after:   `{"0":0,"a":1}`,
			ignored: "/0",
			want:    `{"0":0,"a":1}`,
			wantOps: 1,
		},
		// The ignored value exists in only one document, so the replacement
		// of an ancestor would add or remove it.
		{
			name:    "target only",
			before:  `{"a":0}`,
			after:   `{"a":1,"keep":5}`,
			ignored: "/keep",
			want:    `{"a":1}`,
		},
		{
			name:    "source only",
			before:  `{"a":0,"keep":5}`,
			after:   `{"a":1}`,
			ignored: "/keep",
			want:    `{"a":1,"keep":5}`,
		},
		// A malformed ignored pointer is treated as a difference,
		// so the operations are kept.
		{
			name:    "malformed pointer",
			before:  `{"a":0,"b":0}`,
			after:   `{"a":1,"b":1}`,
			ignored: "/b~",
			want:    `{"a":1,"b":1}`,
			wantOps: 2,
		},
		// The ignored value is inside an Equivalent array, so it is
		// resolved and compared in both documents.
		{
			name:    "equivalent differ",
			before:  `{"a":[1,2],"x":0,"y":0}`,
			after:   `{"a":[2,1],"x":1,"y":1}`,
			ignored: "/a/0",
			opts:    []Option{Equivalent()},
			want:    `{"a":[1,2],"x":1,"y":1}`,
			wantOps: 2,
		},
		{
			name:    "equivalent absent",
			before:  `{"a":[1,2],"x":0,"y":0}`,
			after:   `{"a":[2,1],"x":1,"y":1}`,
			ignored: "/a/5",
			opts:    []Option{Equivalent()},
			want:    `{"a":[2,1],"x":1,"y":1}`,
			wantOps: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]Option{
				Rationalize(),
				Ignores(tc.ignored),
			},
				tc.opts...,
			)
			patch, err := CompareJSON([]byte(tc.before), []byte(tc.after), opts...)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantOps != 0 && len(patch) != tc.wantOps {
				t.Errorf("got %d operations, want %d", len(patch), tc.wantOps)
			}
			got, err := patch.apply([]byte(tc.before), true)
			if err != nil {
				t.Fatal(err)
			}
			if !jsonBytesEqual(t, got, []byte(tc.want)) {
				t.Errorf("applied patch = %s, want %s", got, tc.want)
			}
		})
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

	var i any
	if err := json.Unmarshal(b, &i); err != nil {
		t.Error(err)
	}
	b2, err := json.Marshal(i)
	if err != nil {
		t.Error(err)
	}
	return b2
}

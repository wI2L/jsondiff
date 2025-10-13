package jsondiff

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestPatch_Invert(t *testing.T) {
	t.Run("ambiguous-copy-op", func(t *testing.T) {
		p := Patch{
			Operation{Type: OperationCopy, From: "/foo", Path: "/foo/bar"},
		}
		_, err := p.Invert()
		if !errors.Is(err, ErrAmbiguousCopyOp) {
			t.Errorf("expected ErrAmbiguousCopyOp, got %v", err)
		}
	})
	t.Run("remove-missing-test", func(t *testing.T) {
		p := Patch{
			Operation{Type: OperationRemove, From: "/bar", Path: "/baz"},
		}
		_, err := p.Invert()
		if !errors.Is(err, ErrNonReversible) {
			t.Errorf("expected ErrNonReversible, got %T (%s)", err, err)
		}
	})
	t.Run("replace-missing-test", func(t *testing.T) {
		p := Patch{
			Operation{Type: OperationReplace, From: "/bar", Path: "/baz"},
		}
		_, err := p.Invert()
		if !errors.Is(err, ErrNonReversible) {
			t.Errorf("expected ErrNonReversible, got %T (%s)", err, err)
		}
	})
	t.Run("mismatch-test-pointer", func(t *testing.T) {
		p := Patch{
			Operation{Type: OperationTest, Path: "/a", Value: "1"},
			Operation{Type: OperationReplace, Path: "/b", Value: "3"},
		}
		_, err := p.Invert()

		var testPtrErr *ErrTestPointer
		if errors.As(err, &testPtrErr) {
			if testPtrErr.Op != OperationReplace {
				t.Errorf("expected replace op in error, got %v", testPtrErr.Op)
			}
			if testPtrErr.Error() == "" {
				t.Errorf("expected non empty stringified error")
			}
		} else {
			t.Errorf("expected ErrTestPointer, got %T", err)
		}
	})
	t.Run("object", func(t *testing.T) {
		cases, err := testCasesFromFile(t, "testdata/tests/jsonpatch/object.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cases {
			testInvert(t, c.Before, c.After)
		}
	})
	t.Run("array", func(t *testing.T) {
		cases, err := testCasesFromFile(t, "testdata/tests/jsonpatch/array.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cases {
			testInvert(t, c.Before, c.After)
		}
	})
}

type rawTestCase struct {
	Before json.RawMessage `json:"before"`
	After  json.RawMessage `json:"after"`
}

func testCasesFromFile(t *testing.T, file string) ([]rawTestCase, error) {
	t.Helper()

	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var cases []rawTestCase
	if err := json.Unmarshal(b, &cases); err != nil {
		return nil, err
	}
	return cases, nil
}

func testInvert(t *testing.T, src, tgt []byte) {
	t.Helper()

	t.Run("inverse", func(t *testing.T) {
		p, err := CompareJSON(src, tgt, Invertible())
		if err != nil {
			t.Fatal(err)
		}
		ip, err := p.Invert()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s", ip.String())

		src2, err := ip.apply(tgt, true)
		if err != nil {
			t.Fatal(err)
		}
		if !jsonBytesEqual(t, src, src2) {
			t.Errorf("expected reverted document to equal source")
		}
	})
	t.Run("involution", func(t *testing.T) {
		p, err := CompareJSON(src, tgt, Invertible())
		if err != nil {
			t.Fatal(err)
		}
		ip, err := p.Invert()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s", ip.String())

		iip, err := ip.Invert()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s", iip.String())

		tgt2, err := iip.apply(src, true)
		if err != nil {
			t.Fatal(err)
		}
		if !jsonBytesEqual(t, tgt, tgt2) {
			t.Errorf("expected double inverted document to equal target")
		}
	})
}

func jsonBytesEqual(t *testing.T, a, b []byte) bool {
	t.Helper()

	var (
		aa any
		bb any
	)
	if err := json.Unmarshal(a, &aa); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &bb); err != nil {
		t.Fatal(err)
	}
	return deepEqual(aa, bb)
}

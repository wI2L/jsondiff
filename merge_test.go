package jsondiff

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_mergePatch(t *testing.T) {
	type testcase struct {
		Name string      `json:"name"`
		B    interface{} `json:"before"`
		A    interface{} `json:"after"`
		P    interface{} `json:"patch"`
	}
	for _, testFile := range []string{
		"testdata/tests/mergepatch/rfc.json",
		"testdata/tests/mergepatch/array.json",
		"testdata/tests/mergepatch/object.json",
	} {
		name := strings.TrimSuffix(filepath.Base(testFile), filepath.Ext(testFile))

		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(testFile)
			if err != nil {
				t.Fatal(err)
			}
			var cases []testcase
			if err := json.Unmarshal(b, &cases); err != nil {
				t.Fatal(err)
			}
			for _, tc := range cases {
				t.Run(tc.Name, func(t *testing.T) {
					patch := mergePatch(tc.B, tc.A)
					if !deepEqual(tc.P, patch) {
						t.Errorf("got %v, want %v", patch, tc.P)
						t.Logf("source: %v", tc.B)
						t.Logf("target %v", tc.A)
					}
				})
			}
		})
	}
}

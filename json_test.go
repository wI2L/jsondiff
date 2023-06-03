package jsondiff

import (
	"encoding/json"
	"os"
	"testing"
)

func Test_findKey(t *testing.T) {
	for _, tt := range []struct {
		json string
		key  string
		want string
	}{
		{
			``,
			"foo",
			``,
		},
		{
			`{"a":"foo","b":"bar"}`,
			"b",
			`"bar"`,
		},
		{
			`{"a":[1,2,3],"b":[3,4,5]}`,
			"a",
			`[1,2,3]`,
		},
		{
			`{"":{"a":"b"}}`,
			"",
			`{"a":"b"}`,
		},
	} {
		// Valid JSON input and result.
		if len(tt.json) != 0 && !json.Valid([]byte(tt.json)) {
			t.Errorf("invalid JSON input: %q", tt.json)
		}
		if len(tt.want) != 0 && !json.Valid([]byte(tt.want)) {
			t.Errorf("invalid JSON result: %q", tt.want)
		}
		s := findKey(tt.json, tt.key)
		if s != tt.want {
			t.Errorf("got %q, want %q", s, tt.want)
		}
	}
}

func Test_findIndex(t *testing.T) {
	for _, tt := range []struct {
		json  string
		index int
		want  string
	}{
		{
			``,
			1,
			``,
		},
		{
			`["a","b","c"]`,
			1,
			`"b"`,
		},
		{
			`[1,2,3,4,5]`,
			3,
			`4`,
		},
		{
			`[false,true,"foo","bar"]`,
			3,
			`"bar"`,
		},
		{
			`[["a","b"],[1,2]]`,
			0,
			`["a","b"]`,
		},
		{
			`[["a","b"],[1,2]]`,
			1,
			`[1,2]`,
		},
		{
			`[{"a":"b"},{"c":"d"}]`,
			1,
			`{"c":"d"}`,
		},
		{
			`["\"a","\\b]","\""]`,
			2,
			`"\""`,
		},
		{
			`[["\"a"],["\\\b"]]`,
			0,
			`["\"a"]`,
		},
		{
			`[["\"a"],["fjj\\\"]\""]]`,
			1,
			`["fjj\\\"]\""]`,
		},
		{
			`[[{"a":"1"},{"b":"2"}],[{"c":"3"},{"d":"4"}]]`,
			1,
			`[{"c":"3"},{"d":"4"}]`,
		},
		{
			`[[],""]`,
			0,
			`[]`,
		},
		{
			`[{"a":[1,2,3]},{"b":{"c":[4,5,6]}}]`,
			0,
			`{"a":[1,2,3]}`,
		},
		{
			`[{"a":[1,2,3]},{"b":{"c":[4,5,6]}}]`,
			1,
			`{"b":{"c":[4,5,6]}}`,
		},
		{
			`[]`,
			0,
			``,
		},
	} {
		// Valid JSON input and result.
		if len(tt.json) != 0 && !json.Valid([]byte(tt.json)) {
			t.Errorf("invalid JSON input: %q", tt.json)
		}
		if len(tt.want) != 0 && !json.Valid([]byte(tt.want)) {
			t.Errorf("invalid JSON result: %q", tt.want)
		}
		s := findIndex(tt.json, tt.index)
		if s != tt.want {
			t.Errorf("got %q, want %q", s, tt.want)
		}
	}
}

func Test__compact(t *testing.T) {
	small, err := os.ReadFile("testdata/benchs/small/source.json")
	if err != nil {
		t.Fatal(err)
	}
	b := _compact(small, small)

	const want = `{"pine":true,"silence":{"feathers":"could","lion":false,"provide":["lake",1886677335,"research",false],"ate":"nearest"},"already":true,"it":false}`

	if string(b) != want {
		t.Errorf("got %q, want %q", b, want)
	}
}

package jsondiff

import (
	"reflect"
	"testing"
)

func Test_lcs(t *testing.T) {
	for _, tt := range []struct {
		name  string
		src   []interface{}
		tgt   []interface{}
		pairs [][2]int
	}{
		{
			name: "identical slices",
			src:  []interface{}{"a", "b", "c"},
			tgt:  []interface{}{"a", "b", "c"},
			pairs: [][2]int{
				{0, 0},
				{1, 1},
				{2, 2},
			},
		},
		{
			name: "different slices (expand)",
			src:  []interface{}{"a", "b", "c", "e", "h", "j", "l", "m", "n", "p"},
			tgt:  []interface{}{"b", "c", "d", "e", "f", "j", "k", "l", "m", "r", "s", "t"},
			pairs: [][2]int{
				{1, 0},
				{2, 1},
				{3, 3},
				{5, 5},
				{6, 7},
				{7, 8},
			},
		},
		{
			name: "different slices (shrink)",
			src:  []interface{}{"a", "b", "y", "w", "c"},
			tgt:  []interface{}{"a", "z", "b", "c"},
			pairs: [][2]int{
				{0, 0},
				{1, 2},
				{4, 3},
			},
		},
		{
			name: "slices with duplicates",
			src:  []interface{}{"a", "b", "a", "y", "c", "c"},
			tgt:  []interface{}{"z", "b", "a", "c", "c", "b"},
			pairs: [][2]int{
				{1, 1},
				{2, 2},
				{4, 3},
				{5, 4},
			},
		},
		{
			name:  "all deletions",
			src:   []interface{}{"a", "b", "c", "d"},
			tgt:   []interface{}{},
			pairs: [][2]int{},
		},
		{
			name:  "all additions",
			src:   []interface{}{},
			tgt:   []interface{}{"a", "b", "c", "d"},
			pairs: [][2]int{},
		},
		{
			name:  "all deletions and additions",
			src:   []interface{}{"a", "b", "c", "d"},
			tgt:   []interface{}{"e", "f", "g", "h"},
			pairs: [][2]int{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pairs := lcs(tt.src, tt.tgt)
			if !reflect.DeepEqual(pairs, tt.pairs) {
				t.Errorf("got %v, want %v", pairs, tt.pairs)
			}
		})
	}
}

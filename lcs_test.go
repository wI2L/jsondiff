package jsondiff

import (
	"reflect"
	"testing"
)

func Test_lcs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   []any
		tgt   []any
		pairs [][2]int
	}{
		{
			name: "identical slices",
			src:  []any{"a", "b", "c"},
			tgt:  []any{"a", "b", "c"},
			pairs: [][2]int{
				{0, 0},
				{1, 1},
				{2, 2},
			},
		},
		{
			name: "different slices (expand)",
			src:  []any{"a", "b", "c", "e", "h", "j", "l", "m", "n", "p"},
			tgt:  []any{"b", "c", "d", "e", "f", "j", "k", "l", "m", "r", "s", "t"},
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
			src:  []any{"a", "b", "y", "w", "c"},
			tgt:  []any{"a", "z", "b", "c"},
			pairs: [][2]int{
				{0, 0},
				{1, 2},
				{4, 3},
			},
		},
		{
			name: "slices with duplicates",
			src:  []any{"a", "b", "a", "y", "c", "c"},
			tgt:  []any{"z", "b", "a", "c", "c", "b"},
			pairs: [][2]int{
				{1, 1},
				{2, 2},
				{4, 3},
				{5, 4},
			},
		},
		{
			name:  "all deletions",
			src:   []any{"a", "b", "c", "d"},
			tgt:   []any{},
			pairs: [][2]int{},
		},
		{
			name:  "all additions",
			src:   []any{},
			tgt:   []any{"a", "b", "c", "d"},
			pairs: [][2]int{},
		},
		{
			name:  "all deletions and additions",
			src:   []any{"a", "b", "c", "d"},
			tgt:   []any{"e", "f", "g", "h"},
			pairs: [][2]int{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pairs := lcs(tc.src, tc.tgt)
			if !reflect.DeepEqual(pairs, tc.pairs) {
				t.Errorf("got %v, want %v", pairs, tc.pairs)
			}
		})
	}
}

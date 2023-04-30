package jsondiff

import (
	"encoding/json"
	"reflect"
	"testing"
)

func reflectKind(i interface{}) reflect.Kind {
	return reflect.TypeOf(i).Kind()
}

func BenchmarkGetType(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	m := map[string]interface{}{}

	b.Run("reflect", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = reflectKind(m)
		}
	})
	b.Run("typeSwitch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = typeSwitchKind(m)
		}
	})
}

func Test_typeSwitchKind(t *testing.T) {
	for _, tt := range []struct {
		val   any
		valid bool
		kind  reflect.Kind
	}{
		{
			"foo",
			true,
			reflect.String,
		},
		{
			false,
			true,
			reflect.Bool,
		},
		{
			float32(3.14),
			false,
			reflect.Invalid,
		},
		{
			nil,
			true,
			reflect.Ptr,
		},
		{
			&struct{}{},
			false,
			reflect.Invalid,
		},
		{
			3.14,
			true,
			reflect.Float64,
		},
		{
			func() {},
			false,
			reflect.Invalid,
		},
		{
			[]interface{}{},
			true,
			reflect.Slice,
		},
		{
			map[string]interface{}{},
			true,
			reflect.Map,
		},
		{
			json.Number("3.14"),
			true,
			reflect.String,
		},
	} {
		k := typeSwitchKind(tt.val)
		if k != tt.kind {
			t.Errorf("got %s, want %s", k, tt.kind)
		}
	}
}

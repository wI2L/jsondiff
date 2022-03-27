package jsondiff

import (
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

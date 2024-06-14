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
			_ = jsonTypeSwitch(m)
		}
	})
}

func Test_jsonTypeSwitch(t *testing.T) {
	for _, tc := range []struct {
		val   any
		valid bool
		kind  jsonValueType
	}{
		{
			"foo",
			true,
			jsonString,
		},
		{
			false,
			true,
			jsonBoolean,
		},
		{
			float32(3.14),
			false,
			jsonInvalid,
		},
		{
			nil,
			true,
			jsonNull,
		},
		{
			&struct{}{},
			false,
			jsonInvalid,
		},
		{
			3.14,
			true,
			jsonNumberFloat,
		},
		{
			json.Number("3.14"),
			true,
			jsonNumberString,
		},
		{
			func() {},
			false,
			jsonInvalid,
		},
		{
			[]interface{}{},
			true,
			jsonArray,
		},
		{
			map[string]interface{}{},
			true,
			jsonObject,
		},
	} {
		k := jsonTypeSwitch(tc.val)
		if k != tc.kind {
			t.Errorf("got %s, want %s", k, tc.kind)
		}
	}
}

func Test_deepEqualValue(t *testing.T) {
	for _, tc := range []struct {
		src, tgt interface{}
		equal    bool
	}{
		{
			nil,
			nil,
			true,
		},
		{
			"foobar",
			"foobar",
			true,
		},
		{
			true,
			false,
			false,
		},
		{
			3.14,
			3.14,
			true,
		},
		{
			json.Number("42.69"),
			json.Number("69.42"),
			false,
		},
	} {
		ok := deepEqualValue(tc.src, tc.tgt)
		if ok != tc.equal {
			t.Errorf("got %t, want %t", ok, tc.equal)
		}
	}
}

func Test_deepEqual_invalid_type(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("expected to recover non-nil error")
		}
	}()
	fn := func() {}
	type foo struct{}

	deepEqual(nil, fn)
	deepEqual(foo{}, nil)
}

func Test_jsonValueType_String(t *testing.T) {
	for typ := 0; typ < len(jsonTypeNames); typ++ {
		s := jsonValueType(typ).String()
		if s != jsonTypeNames[typ] {
			t.Errorf("got %q, want %q", s, jsonTypeNames[typ])
		}
	}
	unknownType := jsonValueType(9000)
	s := unknownType.String()

	const want = "type9000"
	if s != want {
		t.Errorf("got %q, want %q", s, want)
	}
}

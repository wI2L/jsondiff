package jsondiff

import "encoding/json"

// An Option overrides the default diff behavior of
// the CompareOpts and CompareJSONOpts function.
type Option func(*differ)

// Compare compares the JSON representations of the
// given values and returns the differences relative
// to the former as a list of JSON Patch operations.
func Compare(source, target interface{}) (Patch, error) {
	var d differ
	return compare(&d, source, target)
}

// CompareOpts is similar to Compare, but also accepts
// a list of options to configure the diff behavior.
func CompareOpts(source, target interface{}, opts ...Option) (Patch, error) {
	var d differ
	d.applyOpts(opts...)

	return compare(&d, source, target)
}

// CompareJSON compares the given JSON documents and
// returns the differences relative to the former as
// a list of JSON Patch operations.
func CompareJSON(source, target []byte) (Patch, error) {
	var d differ
	return compareJSON(&d, source, target)
}

// CompareJSONOpts is similar to CompareJSON, but also
// accepts a list of options to configure the diff behavior.
func CompareJSONOpts(source, target []byte, opts ...Option) (Patch, error) {
	var d differ
	d.applyOpts(opts...)

	return compareJSON(&d, source, target)
}

// Factorize enables factorization of operations.
func Factorize() Option {
	return func(o *differ) { o.factorize = true }
}

// Rationalize enables rationalization of operations.
func Rationalize() Option {
	return func(o *differ) { o.rationalize = true }
}

// Invertible enables the generation of an invertible
// patch, by preceding each remove and replace operation
// by a test operation that verifies the value at the
// path that is being removed/replaced.
// Note that copy operations are not invertible, and as
// such, using this option disable the usage of copy
// operation in favor of add operations.
func Invertible() Option {
	return func(o *differ) { o.invertible = true }
}

func compare(d *differ, src, tgt interface{}) (Patch, error) {
	si, _, err := marshalUnmarshal(src)
	if err != nil {
		return nil, err
	}
	ti, tb, err := marshalUnmarshal(tgt)
	if err != nil {
		return nil, err
	}
	d.targetBytes = tb
	d.diff(si, ti)

	return d.patch, nil
}

func compareJSON(d *differ, src, tgt []byte) (Patch, error) {
	var si, ti interface{}
	if err := json.Unmarshal(src, &si); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(tgt, &ti); err != nil {
		return nil, err
	}
	d.targetBytes = tgt
	d.diff(si, ti)

	return d.patch, nil
}

// marshalUnmarshal returns the result of unmarshaling
// the JSON representation of the given value.
func marshalUnmarshal(i interface{}) (interface{}, []byte, error) {
	b, err := json.Marshal(i)
	if err != nil {
		return nil, nil, err
	}
	var val interface{}
	if err := json.Unmarshal(b, &val); err != nil {
		return nil, nil, err
	}
	return val, b, nil
}

package jsondiff

import (
	"encoding/json"
)

// MergePatch returns a JSON Merge Patch (RFC 7386)
// of the differences between the JSON representations
// of the given values.
func MergePatch(src, tgt interface{}) ([]byte, error) {
	opts := options{
		marshal:   json.Marshal,
		unmarshal: json.Unmarshal,
	}
	si, _, err := marshalUnmarshal(src, opts)
	if err != nil {
		return nil, err
	}
	ti, _, err := marshalUnmarshal(tgt, opts)
	if err != nil {
		return nil, err
	}
	patch := mergePatch(si, ti)
	if patch == nil {
		return nil, nil
	}
	return json.Marshal(patch)
}

// MergePatchJSON compares the given JSON documents
// and returns the differences relative to the former
// as a JSON Merge Patch (RFC 7386)
func MergePatchJSON(src, tgt []byte) ([]byte, error) {
	var si, ti interface{}
	if err := json.Unmarshal(src, &si); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(tgt, &ti); err != nil {
		return nil, err
	}
	patch := mergePatch(si, ti)
	if patch == nil {
		return nil, nil
	}
	return json.Marshal(patch)
}

func mergePatch(src, tgt interface{}) interface{} {
	if src == nil || tgt == nil {
		return tgt
	}
	// If the target is not of the same type as the source,
	// or both are not objects, the patch replaces the entire
	// source with the target.
	// https://datatracker.ietf.org/doc/html/rfc7386#section-2
	if jsonTypeSwitch(src) != jsonObject || jsonTypeSwitch(tgt) != jsonObject {
		return tgt
	}
	sm := src.(map[string]interface{})
	tm := tgt.(map[string]interface{})

	cmpSet := make(map[string]uint8, max(len(sm), len(tm)))

	for k := range sm {
		cmpSet[k] |= 1 << 0
	}
	for k := range tm {
		cmpSet[k] |= 1 << 1
	}
	keys := make([]string, 0, len(cmpSet))
	for k := range cmpSet {
		keys = append(keys, k)
	}
	sortStrings(keys)

	patch := make(map[string]interface{}, len(sm))

	for _, k := range keys {
		v := cmpSet[k]
		inOld := v&(1<<0) != 0
		inNew := v&(1<<1) != 0

		switch {
		case inOld && inNew:
			if !deepEqual(sm[k], tm[k]) {
				patch[k] = mergePatch(sm[k], tm[k])
			}
		case inOld:
			// Null values in the merge patch are given
			// special meaning to indicate the removal
			// of existing values in the target.
			// https://datatracker.ietf.org/doc/html/rfc7386#section-1
			patch[k] = nil
		case inNew:
			patch[k] = tm[k]
		}
	}
	return patch
}

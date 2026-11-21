package jsondiff

import (
	"encoding/binary"
	"encoding/json"
	"hash/maphash"
	"math"
	"slices"
)

type hasher struct {
	mh maphash.Hash
}

func (h *hasher) digest(val any, sort bool) uint64 {
	h.mh.Reset()
	h.hash(val, sort)

	return h.mh.Sum64()
}

// hash writes a type tag before each value, and a length
// before strings, arrays and objects, so that values of
// different types or shapes (such as "" and [], "0" and
// false, [[1]] and [1], or {"x":"y"} and {"xy":""}) do
// not produce the same digest.
func (h *hasher) hash(i any, sort bool) {
	switch v := i.(type) {
	case json.Number:
		_ = h.mh.WriteByte('j')
		h.writeLen(len(v))
		_, _ = h.mh.WriteString(string(v))
	case string:
		_ = h.mh.WriteByte('s')
		h.writeLen(len(v))
		_, _ = h.mh.WriteString(v)
	case bool:
		if v {
			_ = h.mh.WriteByte('t')
		} else {
			_ = h.mh.WriteByte('f')
		}
	case float64:
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], math.Float64bits(v))
		_ = h.mh.WriteByte('n')
		_, _ = h.mh.Write(buf[:])
	case nil:
		_ = h.mh.WriteByte('z')
	case []any:
		_ = h.mh.WriteByte('a')
		h.writeLen(len(v))
		if sort {
			// Sort the digests of the elements instead of the elements themselves.
			// Sorting the slice in place would mutate the documents being compared
			// and corrupt the indexes of the generated patch.
			sub := hasher{}
			sub.mh.SetSeed(h.mh.Seed())
			digests := make([]uint64, len(v))
			for i, e := range v {
				digests[i] = sub.digest(e, true)
			}
			slices.Sort(digests)
			for _, u := range digests {
				var buf [8]byte
				binary.BigEndian.PutUint64(buf[:], u)
				_, _ = h.mh.Write(buf[:])
			}
			return
		}
		for _, e := range v {
			h.hash(e, sort)
		}
	case map[string]any:
		keys := make([]string, 0, len(v))

		// Extract keys first, and sort them
		// in lexicographical order.
		for k := range v {
			keys = append(keys, k)
		}
		sortStrings(keys)

		_ = h.mh.WriteByte('o')
		h.writeLen(len(v))
		for _, k := range keys {
			h.writeLen(len(k))
			_, _ = h.mh.WriteString(k)
			h.hash(v[k], sort)
		}
	}
}

func (h *hasher) writeLen(n int) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(n))
	_, _ = h.mh.Write(buf[:])
}

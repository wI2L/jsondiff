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

func (h *hasher) hash(i any, sort bool) {
	switch v := i.(type) {
	case json.Number:
		_, _ = h.mh.WriteString(string(v))
	case string:
		_, _ = h.mh.WriteString(v)
	case bool:
		if v {
			_ = h.mh.WriteByte('1')
		} else {
			_ = h.mh.WriteByte('0')
		}
	case float64:
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], math.Float64bits(v))
		_, _ = h.mh.Write(buf[:])
	case nil:
		_ = h.mh.WriteByte('0')
	case []any:
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

		for _, k := range keys {
			_, _ = h.mh.WriteString(k)
			h.hash(v[k], sort)
		}
	}
}

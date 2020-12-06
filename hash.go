package jsondiff

import (
	"encoding/binary"
	"hash/maphash"
	"math"
	"sort"
	"strconv"
)

type hasher struct {
	mh maphash.Hash
}

func (h *hasher) digest(val interface{}) uint64 {
	h.mh.Reset()
	h.hash(val)
	return h.mh.Sum64()
}

func (h *hasher) hash(i interface{}) {
	switch v := i.(type) {
	case string:
		h.mh.WriteString(v)
	case bool:
		h.mh.WriteString(strconv.FormatBool(v))
	case float64:
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], math.Float64bits(v))
		h.mh.Write(buf[:])
	case nil:
		h.mh.WriteString("nil")
	case []interface{}:
		for i, e := range v {
			h.mh.WriteString(strconv.Itoa(i))
			h.hash(e)
		}
	case map[string]interface{}:
		// Extract keys first, and sort them
		// in lexicographical order.
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			h.mh.WriteString(k)
			h.hash(v[k])
		}
	}
}

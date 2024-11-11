package jsondiff

import (
	"sort"
	"strings"
	"unsafe"
)

// A Differ generates JSON Patch (RFC 6902).
// The zero value is an empty generator ready to use.
type Differ struct {
	hashmap          map[uint64]jsonNode
	opts             options
	patch            Patch
	snapshotPatchLen int
	targetBytes      []byte
	ptr              pointer
	hasher           hasher
	isCompact        bool
	compactInPlace   bool
}

type (
	marshalFunc   func(any) ([]byte, error)
	unmarshalFunc func([]byte, any) error
)

type options struct {
	ignores     map[string]struct{}
	marshal     marshalFunc
	unmarshal   unmarshalFunc
	hasIgnore   bool
	factorize   bool
	rationalize bool
	invertible  bool
	equivalent  bool
	lcs         bool
}

type jsonNode struct {
	val any
	ptr string
}

// Reset resets the Differ to be empty, but it retains the
// underlying storage for use by future comparisons.
func (d *Differ) Reset() {
	d.patch = d.patch[:0]
	d.ptr.reset()

	// Optimized map clear.
	for k := range d.hashmap {
		delete(d.hashmap, k)
	}
}

// WithOpts applies the given options to the Differ
// instance and returns it to allow chained calls.
func (d *Differ) WithOpts(opts ...Option) *Differ {
	for _, o := range opts {
		o(d)
	}
	return d
}

// Patch returns the list of JSON patch operations
// generated by the Differ instance. The patch is
// valid for usage until the next comparison or reset.
func (d *Differ) Patch() Patch {
	return d.patch
}

// Compare computes the differences between src and tgt
// as a series of JSON Patch operations.
func (d *Differ) Compare(src, tgt interface{}) {
	if d.opts.factorize {
		d.prepare(d.ptr, src, tgt)
		d.ptr.reset()
	}
	if d.opts.rationalize {
		if !d.isCompact {
			if d.compactInPlace {
				d.targetBytes = compactInPlace(d.targetBytes)
			} else {
				d.targetBytes = compact(d.targetBytes)
			}
		}
	}
	d.diff(d.ptr, src, tgt, b2s(d.targetBytes))
}

func (d *Differ) isIgnored(ptr pointer) bool {
	// Fast path, inlined map check.
	if !d.opts.hasIgnore {
		return false
	}
	// Slow path.
	// Outlined so that the fast path can be inlined.
	return d.findIgnored(ptr)
}

func (d *Differ) findIgnored(ptr pointer) bool {
	_, found := d.opts.ignores[ptr.string()]
	return found
}

func (d *Differ) diff(ptr pointer, src, tgt interface{}, doc string) {
	if d.isIgnored(ptr) {
		return
	}
	if !areComparable(src, tgt) {
		if ptr.isRoot() {
			// If incomparable values are located at the root
			// of the document, use an add operation to replace
			// the entire content of the document.
			// https://tools.ietf.org/html/rfc6902#section-4.1
			d.patch = d.patch.append(OperationAdd, emptyPointer, ptr.copy(), src, tgt, 0)
		} else {
			// Values are incomparable, generate a replacement.
			d.replace(ptr.copy(), src, tgt, doc)
		}
		return
	}
	if deepEqual(src, tgt) {
		return
	}
	// Save the current size of the patch to detect later
	// on if we have new operations to rationalize.
	size := len(d.patch)

	// Values are comparable, but are not
	// equivalent.
	switch val := src.(type) {
	case []interface{}:
		if d.opts.lcs {
			d.compareArraysLCS(ptr, val, tgt.([]interface{}), doc)
		} else {
			d.compareArrays(ptr, val, tgt.([]interface{}), doc)
		}
	case map[string]interface{}:
		d.compareObjects(ptr, val, tgt.(map[string]interface{}), doc)
	default:
		// Generate a replace operation for
		// scalar types.
		if !deepEqual(src, tgt) {
			d.replace(ptr.copy(), src, tgt, doc)
			return
		}
	}
	// Rationalize new operations, if any.
	if d.opts.rationalize && len(d.patch) > size {
		d.rationalize(ptr, src, tgt, size, doc)
	}
}

func (d *Differ) prepare(ptr pointer, src, tgt interface{}) {
	// When both values are deeply equals, save
	// the location indexed by the value hash.
	if !areComparable(src, tgt) {
		return
	} else if deepEqual(src, tgt) {
		k := d.hasher.digest(tgt, false)
		if d.hashmap == nil {
			d.hashmap = make(map[uint64]jsonNode)
		}
		d.hashmap[k] = jsonNode{
			ptr: ptr.copy(),
			val: tgt,
		}
		return
	}
	// At this point, the source and target values
	// are non-nil and have comparable types.
	switch vsrc := src.(type) {
	case []interface{}:
		oarr := vsrc
		narr := tgt.([]interface{})

		for i := 0; i < min(len(oarr), len(narr)); i++ {
			p := ptr.clone()
			p.appendIndex(i)
			d.prepare(p, oarr[i], narr[i])
		}
	case map[string]interface{}:
		oobj := vsrc
		nobj := tgt.(map[string]interface{})

		for k, v1 := range oobj {
			if v2, ok := nobj[k]; ok {
				p := ptr.clone()
				p.appendKey(k)
				d.prepare(p, v1, v2)
			}
		}
	default:
		// Skipped.
	}
}

func (d *Differ) rationalize(ptr pointer, src, tgt interface{}, lastOpIdx int, doc string) {
	// replaceOp represents a single operation that
	// replace the source document with the target.
	replaceOp := Operation{
		Type:     OperationReplace,
		Path:     ptr.string(), // shallow copy
		OldValue: src,
		Value:    tgt,
		valueLen: len(doc),
	}
	curOps := d.patch[lastOpIdx:]
	curLen := curOps.jsonLength()

	// If one operation is cheaper than many small
	// operations that represents the changes between
	// the two objects, replace the last operations.
	if curLen > replaceOp.jsonLength() {
		d.patch = d.patch[:lastOpIdx]

		// Allocate a new string for the operation's path.
		replaceOp.Path = ptr.copy()

		if d.opts.invertible {
			d.patch = d.patch.append(OperationTest, emptyPointer, replaceOp.Path, nil, src, len(doc))
		}
		d.patch = append(d.patch, replaceOp)
	}
}

// compareObjects generates the patch operations that
// represents the differences between two JSON objects.
func (d *Differ) compareObjects(ptr pointer, src, tgt map[string]interface{}, doc string) {
	cmpSet := make(map[string]uint8, max(len(src), len(tgt)))

	for k := range src {
		cmpSet[k] |= 1 << 0
	}
	for k := range tgt {
		cmpSet[k] |= 1 << 1
	}
	keys := make([]string, 0, len(cmpSet))

	for k := range cmpSet {
		keys = append(keys, k)
	}
	sortStrings(keys)

	ptr.snapshot()
	for _, k := range keys {
		v := cmpSet[k]
		inOld := v&(1<<0) != 0
		inNew := v&(1<<1) != 0

		ptr.appendKey(k)

		switch {
		case inOld && inNew:
			if d.opts.rationalize {
				d.diff(ptr, src[k], tgt[k], findKey(doc, ptr.base.key))
			} else {
				d.diff(ptr, src[k], tgt[k], doc)
			}
		case inOld:
			if !d.isIgnored(ptr) {
				d.remove(ptr.copy(), src[k])
			}
		case inNew:
			if !d.isIgnored(ptr) {
				d.add(ptr.copy(), tgt[k], doc, false)
			}
		}
		ptr.rewind()
	}
}

// compareArrays generates the patch operations that
// represents the differences between two JSON arrays.
func (d *Differ) compareArrays(ptr pointer, src, tgt []interface{}, doc string) {
	ptr.snapshot()
	sl, tl := len(src), len(tgt)
	ml := min(sl, tl)

	// When the source array contains more elements
	// than the target, entries are being removed
	// from the destination and the removal index
	// is always equal to the original array length.
	if tl < sl {
		np := ptr.clone()
		np.appendIndex(ml) // "removal" path
		p := np.copy()
		for i := ml; i < sl; i++ {
			ptr.appendIndex(i)

			if !d.isIgnored(ptr) {
				d.remove(p, src[i])
			}
			ptr.rewind()
		}
		goto comparisons // skip equivalence test since arrays are different
	}
	if d.opts.equivalent && d.unorderedDeepEqualSlice(src, tgt) {
		return
	}
comparisons:
	// Compare the elements at each index present in
	// both the source and destination arrays.
	for i := 0; i < ml; i++ {
		ptr.appendIndex(i)
		if d.opts.rationalize {
			d.diff(ptr, src[i], tgt[i], findIndex(doc, ptr.base.idx))
		} else {
			d.diff(ptr, src[i], tgt[i], doc)
		}
		ptr.rewind()
	}
	// When the target array contains more elements
	// than the source, entries are appended to the
	// destination.
	if tl > sl {
		np := ptr.clone()
		np.appendKey("-") // "append" path
		p := np.copy()
		for i := ml; i < tl; i++ {
			ptr.appendIndex(i)
			if !d.isIgnored(ptr) {
				d.add(p, tgt[i], doc, false)
			}
			ptr.rewind()
		}
	}
}

func (d *Differ) compareArraysLCS(ptr pointer, src, tgt []interface{}, doc string) {
	ptr.snapshot()
	pairs := lcs(src, tgt)
	d.snapshotPatchLen = len(d.patch)

	var ai, bi int // src && tgt arrows
	var add, remove int

	adjust := func(i int) int {
		// Adjust indice considering add and remove
		// operations that precede it.
		return i + add - remove
	}

	// Iterate over all the indices of the LCS, which
	// represent the position of items that are present
	// in both the source and target slices.
	for p := 0; p < len(pairs); p++ {
		ma, mb := pairs[p][0], pairs[p][1]

		// Proceed with addition/deletion or change events
		// until both arrows reach the current indice.
		for ai < ma || bi < mb {
			switch {
			case ai < ma && bi < mb:
				// Both arrows points to an item before the
				// current match indice, which indicate an
				// equal amount of different items.
				ptr.appendIndex(adjust(ai))
				if d.opts.rationalize {
					d.diff(ptr, src[ai], tgt[bi], findIndex(doc, ptr.base.idx))
				} else {
					d.diff(ptr, src[ai], tgt[bi], doc)
				}
				ptr.rewind()
				ai++
				bi++
			case ai < ma:
				// The left arrow representing the source slice
				// is lower than the current match indice, which
				// indicate that a preceding item has been removed.
				ptr.appendIndex(adjust(ai))

				if !d.isIgnored(ptr) {
					d.remove(ptr.copy(), src[ai])
				}
				ptr.rewind()
				ai++
				remove++
			default: // bi < mb
				// Opposite case of the previous condition.
				ptr.appendIndex(bi)
				if !d.isIgnored(ptr) {
					d.add(ptr.copy(), tgt[bi], doc, true)
				}
				ptr.rewind()
				bi++
				add++
			}
		}
		// Both arrows reached the current match indice
		// where the elements of the source and target
		// slice are equal, i.e. `src[ai] == tgt[bi]`.
		ai++
		bi++
	}
	// After all index pairs of the LCS have been traversed,
	// the remaining items up to the length of the source and
	// target slices are iterated. The same logic applies to
	// detect addition/deletion or change events.
	for ai < len(src) || bi < len(tgt) {
		switch {
		case ai < len(src) && bi < len(tgt):
			ptr.appendIndex(adjust(ai))
			if d.opts.rationalize {
				d.diff(ptr, src[ai], tgt[bi], findIndex(doc, ptr.base.idx))
			} else {
				d.diff(ptr, src[ai], tgt[bi], doc)
			}
			ptr.rewind()
			ai++
			bi++
		case ai < len(src):
			ptr.appendIndex(adjust(ai))

			if !d.isIgnored(ptr) {
				d.remove(ptr.copy(), src[ai])
			}
			ptr.rewind()
			ai++
			remove++
		default: // bi < len(tgt)
			ptr.appendIndex(bi)
			if !d.isIgnored(ptr) {
				d.add(ptr.copy(), tgt[bi], doc, true)
			}
			ptr.rewind()
			bi++
			add++
		}
	}
}

func (d *Differ) unorderedDeepEqualSlice(src, tgt []interface{}) bool {
	if len(src) != len(tgt) {
		return false
	}
	diff := make(map[uint64]struct{}, len(src))
	count := 0

	for _, v := range src {
		k := d.hasher.digest(v, d.opts.equivalent)
		diff[k] = struct{}{}
		count++
	}
	for _, v := range tgt {
		k := d.hasher.digest(v, d.opts.equivalent)
		// If the digest hash is not in the comparison set,
		// return early.
		if _, ok := diff[k]; !ok {
			return false
		}
		count--
	}
	return count == 0
}

func (d *Differ) replace(path string, src, tgt interface{}, doc string) {
	vl := len(doc)

	if d.opts.invertible {
		d.patch = d.patch.append(OperationTest, emptyPointer, path, nil, src, vl)
	}
	d.patch = d.patch.append(OperationReplace, emptyPointer, path, src, tgt, vl)
}

func (d *Differ) add(path string, v interface{}, doc string, lcs bool) {
	if !d.opts.factorize {
		d.patch = d.patch.append(OperationAdd, emptyPointer, path, nil, v, 0)
		return
	}
	idx := d.findRemoved(v)
	if idx != -1 {
		op := d.patch[idx]

		// https://tools.ietf.org/html/rfc6902#section-4.4f
		// The "from" location MUST NOT be a proper prefix
		// of the "path" location; i.e., a location cannot
		// be moved into one of its children.
		if !strings.HasPrefix(path, op.Path) {
			d.patch = d.patch.remove(idx)
			if !lcs {
				d.patch = d.patch.append(OperationMove, op.Path, path, v, v, 0)
			} else {
				d.patch = d.patch.insert(d.snapshotPatchLen, OperationMove, op.Path, path, v, v, 0)
			}
		}
		return
	}
	uptr := d.findUnchanged(v)

	if len(uptr) != 0 && !d.opts.invertible {
		d.patch = d.patch.append(OperationCopy, uptr, path, nil, v, 0)
	} else {
		d.patch = d.patch.append(OperationAdd, emptyPointer, path, nil, v, len(doc))
	}
}

func (d *Differ) remove(path string, v interface{}) {
	if d.opts.invertible {
		d.patch = d.patch.append(OperationTest, emptyPointer, path, nil, v, 0)
	}
	d.patch = d.patch.append(OperationRemove, emptyPointer, path, v, nil, 0)
}

func (d *Differ) findUnchanged(v interface{}) string {
	if d.hashmap != nil {
		k := d.hasher.digest(v, false)
		node, ok := d.hashmap[k]
		if ok {
			return node.ptr
		}
	}
	return emptyPointer
}

func (d *Differ) findRemoved(v interface{}) int {
	for i := 0; i < len(d.patch); i++ {
		op := d.patch[i]
		if op.Type == OperationRemove && deepEqual(op.OldValue, v) {
			return i
		}
	}
	return -1
}

func (d *Differ) applyOpts(opts ...Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(d)
		}
	}
}

func sortStrings(v []string) {
	if len(v) <= 20 {
		insertionSort(v)
	} else {
		sort.Strings(v)
	}
}

func insertionSort(v []string) {
	for i := 0; i < len(v); i++ {
		for j := i; j > 0 && v[j-1] > v[j]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

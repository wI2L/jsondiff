package jsondiff

import (
	"sort"
	"strings"
)

type differ struct {
	patch       Patch
	hasher      hasher
	hashmap     map[uint64]jsonNode
	factorize   bool
	rationalize bool
	invertible  bool
	targetBytes []byte
}

func (d *differ) diff(src, tgt interface{}) {
	if d.factorize {
		d.prepare(emptyPtr, src, tgt)
	}
	d.compare(emptyPtr, src, tgt)
}

func (d *differ) compare(ptr pointer, src, tgt interface{}) {
	if src == nil && tgt == nil {
		return
	}
	if !areComparable(src, tgt) {
		if ptr.isRoot() {
			// If incomparable values are located at the root
			// of the document, use an add operation to replace
			// the entire content of the document.
			// https://tools.ietf.org/html/rfc6902#section-4.1
			d.patch = d.patch.append(OperationAdd, emptyPtr, ptr, src, tgt)
		} else {
			// Values are incomparable, generate a replacement.
			d.replace(ptr, src, tgt)
		}
		return
	}
	if deepValueEqual(src, tgt, typeSwitchKind(src)) {
		return
	}
	size := len(d.patch)

	// Values are comparable, but are not
	// equivalent.
	switch val := src.(type) {
	case []interface{}:
		d.compareArrays(ptr, val, tgt.([]interface{}))
	case map[string]interface{}:
		d.compareObjects(ptr, val, tgt.(map[string]interface{}))
	default:
		// Generate a replace operation for
		// scalar types.
		if !deepValueEqual(src, tgt, typeSwitchKind(src)) {
			d.replace(ptr, src, tgt)
			return
		}
	}
	// Rationalize any new operations.
	if d.rationalize && len(d.patch) > size {
		d.rationalizeLastOps(ptr, src, tgt, size)
	}
}

func (d *differ) prepare(ptr pointer, src, tgt interface{}) {
	if src == nil && tgt == nil {
		return
	}
	// When both values are deeply equals, save
	// the location indexed by the value hash.
	if !areComparable(src, tgt) {
		return
	} else if deepValueEqual(src, tgt, typeSwitchKind(src)) {
		k := d.hasher.digest(tgt)
		if d.hashmap == nil {
			d.hashmap = make(map[uint64]jsonNode)
		}
		d.hashmap[k] = jsonNode{ptr: ptr, val: tgt}
		return
	}
	// At this point, the source and target values
	// are non-nil and have comparable types.
	switch vsrc := src.(type) {
	case []interface{}:
		oarr := vsrc
		narr := tgt.([]interface{})

		for i := 0; i < min(len(oarr), len(narr)); i++ {
			d.prepare(ptr.appendIndex(i), oarr[i], narr[i])
		}
	case map[string]interface{}:
		oobj := vsrc
		nobj := tgt.(map[string]interface{})

		for k, v1 := range oobj {
			if v2, ok := nobj[k]; ok {
				d.prepare(ptr.appendKey(k), v1, v2)
			}
		}
	default:
		// Skipped.
	}
}

func (d *differ) rationalizeLastOps(ptr pointer, src, tgt interface{}, lastOpIdx int) {
	newOps := make(Patch, 0, 2)

	if d.invertible {
		newOps = newOps.append(OperationTest, emptyPtr, ptr, nil, src)
	}
	// replaceOp represents a single operation that
	// replace the source document with the target.
	replaceOp := Operation{
		Type:  OperationReplace,
		Path:  ptr,
		Value: tgt,
	}
	newOps = append(newOps, replaceOp)
	curOps := d.patch[lastOpIdx:]

	newLen := replaceOp.jsonLength(d.targetBytes)
	curLen := curOps.jsonLength(d.targetBytes)

	// If one operation is cheaper than many small
	// operations that represents the changes between
	// the two objects, replace the last operations.
	if curLen > newLen {
		d.patch = d.patch[:lastOpIdx]
		d.patch = append(d.patch, newOps...)
	}
}

// compareObjects generates the patch operations that
// represents the differences between two JSON objects.
func (d *differ) compareObjects(ptr pointer, src, tgt map[string]interface{}) {
	cmpSet := make(map[string]uint8)

	for k := range src {
		cmpSet[k] |= 1 << 0
	}
	for k := range tgt {
		cmpSet[k] |= 1 << 1
	}
	for _, k := range sortedObjectKeys(cmpSet) {
		v := cmpSet[k]
		inOld := v&(1<<0) != 0
		inNew := v&(1<<1) != 0

		switch {
		case inOld && inNew:
			d.compare(ptr.appendKey(k), src[k], tgt[k])
		case inOld && !inNew:
			d.remove(ptr.appendKey(k), src[k])
		case !inOld && inNew:
			d.add(ptr.appendKey(k), tgt[k])
		}
	}
}

// compareArrays generates the patch operations that
// represents the differences between two JSON arrays.
func (d *differ) compareArrays(ptr pointer, src, dst []interface{}) {
	size := min(len(src), len(dst))

	// When the source array contains more elements
	// than the target, entries are being removed
	// from the destination and the removal index
	// is always equal to the original array length.
	for i := size; i < len(src); i++ {
		d.remove(ptr.appendIndex(size), src[i])
	}
	// Compare the elements at each index present in
	// both the source and destination arrays.
	for i := 0; i < size; i++ {
		d.compare(ptr.appendIndex(i), src[i], dst[i])
	}
	// When the target array contains more elements
	// than the source, entries are appended to the
	// destination.
	for i := size; i < len(dst); i++ {
		d.add(ptr.appendKey("-"), dst[i])
	}
}

func (d *differ) add(ptr pointer, v interface{}) {
	if !d.factorize {
		d.patch = d.patch.append(OperationAdd, emptyPtr, ptr, nil, v)
		return
	}
	idx := d.findRemoved(v)
	if idx != -1 {
		op := d.patch[idx]

		// https://tools.ietf.org/html/rfc6902#section-4.4
		// The "from" location MUST NOT be a proper prefix
		// of the "path" location; i.e., a location cannot
		// be moved into one of its children.
		if !strings.HasPrefix(string(ptr), string(op.Path)) {
			d.patch = d.patch.remove(idx)
			d.patch = d.patch.append(OperationMove, op.Path, ptr, v, v)
		}
		return
	}
	uptr := d.findUnchanged(v)
	if !uptr.isRoot() && !d.invertible {
		d.patch = d.patch.append(OperationCopy, uptr, ptr, nil, v)
	} else {
		d.patch = d.patch.append(OperationAdd, emptyPtr, ptr, nil, v)
	}
}

// areComparable returns whether the interface values
// i1 and i2 can be compared. The values are comparable
// only if they are both non-nil and share the same kind.
func areComparable(i1, i2 interface{}) bool {
	return typeSwitchKind(i1) == typeSwitchKind(i2)
}

func (d *differ) replace(ptr pointer, src, tgt interface{}) {
	if d.invertible {
		d.patch = d.patch.append(OperationTest, emptyPtr, ptr, nil, src)
	}
	d.patch = d.patch.append(OperationReplace, emptyPtr, ptr, src, tgt)
}

func (d *differ) remove(ptr pointer, v interface{}) {
	if d.invertible {
		d.patch = d.patch.append(OperationTest, emptyPtr, ptr, nil, v)
	}
	d.patch = d.patch.append(OperationRemove, emptyPtr, ptr, v, nil)
}

func (d *differ) findUnchanged(v interface{}) pointer {
	if d.hashmap != nil {
		k := d.hasher.digest(v)
		node, ok := d.hashmap[k]
		if ok {
			return node.ptr
		}
	}
	return emptyPtr
}

func (d *differ) findRemoved(v interface{}) int {
	for i := 0; i < len(d.patch); i++ {
		op := d.patch[i]
		if op.Type == OperationRemove && deepEqual(op.OldValue, v) {
			return i
		}
	}
	return -1
}

func (d *differ) applyOpts(opts ...Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(d)
		}
	}
}

func sortedObjectKeys(m map[string]uint8) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

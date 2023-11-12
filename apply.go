package jsondiff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// apply applies the patch to the given source document.
// If valid is true, the document is validated prior to
// the application of the patch.
func (p Patch) apply(src []byte, valid bool) ([]byte, error) {
	if valid && !json.Valid(src) {
		return nil, fmt.Errorf("invalid source document")
	}
	// Make a copy of the source document which
	// will receive the patch mutations.
	tgt := bytes.Clone(src)

	for _, op := range p {
		dp, err := toDotPath(op.Path, src)
		if err != nil {
			return nil, err
		}
		switch op.Type {
		case OperationAdd:
			tgt, err = add(tgt, dp, op.Value)
		case OperationRemove:
			tgt, err = sjson.DeleteBytes(tgt, dp)
		case OperationReplace:
			tgt, err = replace(tgt, dp, op.Value)
		case OperationMove, OperationCopy:
			// First fetch the value from the source path,
			// and then add it to the destination path.
			fp, err := toDotPath(op.From, src)
			if err != nil {
				return nil, err
			}
			tgt, err = add(tgt, dp, op.Value)
			if err != nil {
				break // bail out to interpret error
			}
			// Finally, if the operation was a move, the
			// source value must be deleted.
			if op.Type == OperationMove {
				tgt, err = sjson.DeleteBytes(tgt, fp)
			}
		case OperationTest:
			r := gjson.GetBytes(tgt, dp)
			if !r.Exists() {
				return nil, fmt.Errorf("invalid patch: %q value is not set", op.Path)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("failed to apply op: %w", err)
		}
	}
	return tgt, nil
}

func replace(tgt []byte, path string, val interface{}) ([]byte, error) {
	if path == "@this" {
		return json.Marshal(val)
	}
	return sjson.SetBytesOptions(tgt, path, val, &sjson.Options{
		Optimistic:     true,
		ReplaceInPlace: true,
	})
}

func add(tgt []byte, path string, val interface{}) ([]byte, error) {
	if path == "@this" {
		// Unsupported by the sjson package.
		// Since an empty path represent the root
		// document, we can simply marshal the value
		// and return it as-is.
		return json.Marshal(val)
	}
	// If we're dealing with an array indice, we want to
	// "insert" the element instead of replacing it.
	// We insert a null value manually where the new element
	// is supposed to be (before the current element), and
	// finally replace the placeholder with the new value.
	if isArrayIndex(path) {
		r := gjson.GetBytes(tgt, path)
		if r.Index > 0 {
			tgt = append(tgt[:r.Index], append([]byte(`null,`), tgt[r.Index:]...)...)
		}
	}
	return sjson.SetBytesOptions(tgt, path, val, &sjson.Options{ReplaceInPlace: true})
}

func isArrayIndex(path string) bool {
	i := strings.LastIndexByte(path, '.')
	if i == -1 {
		if path != "" && unicode.IsDigit(rune(path[0])) {
			return true
		}
		return false
	}
	if i != 0 && path[i-1] == '\\' {
		return false
	}
	if i < len(path) && unicode.IsDigit(rune(path[i+1])) {
		return true
	}
	return false
}

// dotPath converts the given JSON Pointer string to the
// dot-path notation used by tidwall/sjson package.
// The source document is required in order to distinguish
// // numeric object keys from array indices
func toDotPath(path string, src []byte) (string, error) {
	if path == "" {
		// @this returns the current element.
		// It is used to retrieve the root element.
		return "@this", nil
	}
	fragments, err := parsePointer(path)
	if err != nil {
		return "", fmt.Errorf("failed to parse path: %w", err)
	}
	sb := strings.Builder{}

	for i, f := range fragments {
		var key string
		if len(f) != 0 && unicode.IsDigit(rune(f[0])) {
			// The fragment starts with a digit, which
			// indicate that it might be a number.
			if _, err := strconv.ParseInt(f, 10, 64); err == nil {
				// The number is valid, but it could either be an
				// array indice or an object key.
				// Since the JSON Pointer RFC does not differentiate
				// between the two, we have to look up the value to
				// know what we're dealing with.
				p := sb.String()
				if p == "" {
					p = "@this"
				}
				r := gjson.GetBytes(src, p)
				switch {
				case r.IsArray():
					// Write array indice as-is.
					key = f
				case r.IsObject():
					// Force the number as an object key, by
					// preceding it with a colon character.
					key = ":" + f
				default:
					return "", fmt.Errorf("unexpected value type at path: %s", sb.String())
				}
			}
		} else if f == "-" && i == len(fragments)-1 {
			// If the last fragment is the "-" character,
			// it indicates that the value is a nonexistent
			// element to append to the array.
			key = "-1"
		} else {
			key = rfc6901Unescaper.Replace(f)
			key = strings.Replace(key, ".", `\.`, -1)
		}
		if i != 0 {
			// Add separator character
			sb.WriteByte('.')
		}
		sb.WriteString(key)
	}
	return sb.String(), nil
}
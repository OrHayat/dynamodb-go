package serializer

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
)

type IsZeroer interface {
	IsZero() bool
}

var isZeroerType = reflect.TypeOf((*IsZeroer)(nil)).Elem()

func getIsEmptyForType(typ reflect.Type) func(va addressableValue) bool {
	switch typ.Kind() {
	case reflect.String, reflect.Map, reflect.Array, reflect.Slice:
		return func(va addressableValue) bool { return va.Len() == 0 }
	case reflect.Pointer, reflect.Interface:
		return func(va addressableValue) bool { return va.IsNil() }
	}
	return nil
}

func getIsZeroForType(typ reflect.Type) func(va addressableValue) bool {
	// Provide a function that uses a type's IsZero method.
	switch {
	case typ.Kind() == reflect.Interface && typ.Implements(isZeroerType):
		return func(va addressableValue) bool {
			// Avoid panics calling IsZero on a nil interface or
			// non-nil interface with nil pointer.
			return va.IsNil() || (va.Elem().Kind() == reflect.Pointer && va.Elem().IsNil()) || va.Interface().(IsZeroer).IsZero()
		}
	case typ.Kind() == reflect.Pointer && typ.Implements(isZeroerType):
		return func(va addressableValue) bool {
			// Avoid panics calling IsZero on nil pointer.
			return va.IsNil() || va.Interface().(IsZeroer).IsZero()
		}
	case typ.Implements(isZeroerType):
		return func(va addressableValue) bool { return va.Interface().(IsZeroer).IsZero() }
	case reflect.PointerTo(typ).Implements(isZeroerType):
		return func(va addressableValue) bool {
			return va.Addr().Interface().(IsZeroer).IsZero()
		}
	default:
		return func(va addressableValue) bool {
			return va.IsZero()
		}
	}
}

type structFields struct {
	flattened    []structField // listed in depth-first ordering
	byActualName map[string]*structField
}

type structField struct {
	id                   int   // unique numeric ID in breadth-first ordering
	index                []int // index into a struct according to reflect.Type.FieldByIndex
	typ                  reflect.Type
	serializersFunctions *serializer
	isZero               func(addressableValue) bool
	isEmpty              func(addressableValue) bool
	fieldOptions
}

type sfCacheKey struct {
	typ reflect.Type
	tag string
}

var structFieldsCache = sync.Map{}

func makeStructFieldsCached(typ reflect.Type, tag string) (structFields, *SerializerError) {
	key := sfCacheKey{typ: typ, tag: tag}
	val, ok := structFieldsCache.Load(key)
	if ok {
		return val.(structFields), nil
	}
	sf, err := makeStructFields(typ, tag)
	if err != nil {
		return sf, err
	}
	res, _ := structFieldsCache.LoadOrStore(key, sf)
	sf = res.(structFields)
	return sf, nil
}

func makeStructFields(root reflect.Type, tag string) (structFields, *SerializerError) {
	// Setup a queue for a breath-first search.
	var queueIndex int
	type queueEntry struct {
		typ           reflect.Type
		index         []int
		visitChildren bool // whether to recursively visit inlined field in this struct
	}
	queue := []queueEntry{{root, nil, true}}
	seen := map[reflect.Type]bool{root: true}

	// Perform a breadth-first search over all reachable fields.
	// This ensures that len(f.index) will be monotonically increasing.
	var allFields []structField
	for queueIndex < len(queue) {
		qe := queue[queueIndex]
		queueIndex++

		t := qe.typ
		namesIndex := make(map[string]int) // index of each field with a given JSON object name in current struct
		var hasAnyDynamoTag bool           // whether any Go struct field has a tag
		var hasAnyDynamoField bool         // whether any serializable fields exist in current struct
		for i := 0; i < t.NumField(); i++ {
			sf := t.Field(i)
			_, hasTag := sf.Tag.Lookup(tag)
			hasAnyDynamoTag = hasAnyDynamoTag || hasTag
			options, ignored, err := parseFieldOptions(sf, tag)
			if err != nil {
				return structFields{}, newSerializerError(SerializerErrorData{Err: err, GoType: t})
			} else if ignored {
				continue
			}
			hasAnyDynamoField = true
			f := structField{
				// Allocate a new slice (len=N+1) to hold both
				// the parent index (len=N) and the current index (len=1).
				// Do this to avoid clobbering the memory of the parent index.
				index:        append(append(make([]int, 0, len(qe.index)+1), qe.index...), i),
				typ:          sf.Type,
				fieldOptions: options,
			}
			if sf.Anonymous && !f.HasName {
				f.Inline = true // implied by use of Go embedding without an explicit name
			}
			if f.Inline {
				tf := f.typ
				// Handle an inlined field that serializes to/from
				// Unwrap one level of pointer indirection similar to how Go
				// only allows embedding either T or *T, but not **T.
				if tf.Kind() == reflect.Pointer && tf.Name() == "" {
					tf = tf.Elem()
				}

				// Reject any types with custom serialization otherwise
				// it becomes impossible to know what sub-fields to inline.
				if which, _ := implementsWhich(tf, attributeValueMarshalerType); which != nil {
					err = fmt.Errorf("inline field %s of type %s must not implement custom marshal or unmarshal", sf.Name, tf.String())
					return structFields{}, newSerializerError(SerializerErrorData{Err: err, GoType: t})
				}
				// Handle an inlined field that serializes to/from
				// a finite number of JSON object members backed by a Go struct.
				if tf.Kind() == reflect.Struct {
					if qe.visitChildren {
						queue = append(queue, queueEntry{tf, f.index, !seen[tf]})
					}
					seen[tf] = true
					continue
				} else {
					err = fmt.Errorf("inlined Go struct field %s of type %s must be a Go struct", sf.Name, tf)
					return structFields{}, newSerializerError(SerializerErrorData{Err: err, GoType: t})
				}
			} else {
				if f.fieldOptions.OmitZero {
					f.isZero = getIsZeroForType(sf.Type)
				}
				if f.OmitEmpty {
					f.isEmpty = getIsEmptyForType(sf.Type)
				}
				// Reject multiple fields with same name within the same struct.
				if j, ok := namesIndex[f.Name]; ok {
					err = fmt.Errorf("Go struct fields %s and %s conflict over %s object name %q", t.Field(j).Name, sf.Name, tag, f.Name)
					return structFields{}, newSerializerError(SerializerErrorData{Err: err, GoType: t})
				}
				namesIndex[f.Name] = i
				f.id = len(allFields)
				f.serializersFunctions = lookupSerializer(sf.Type)
				allFields = append(allFields, f)
			}
		}
		isEmptyStruct := t.NumField() == 0
		if !isEmptyStruct && !hasAnyDynamoTag && !hasAnyDynamoField {
			err := fmt.Errorf("Go struct %s has no exported fields", t.String())
			return structFields{}, newSerializerError(SerializerErrorData{Err: err, GoType: t})
		}
	}
	// Sort the fields by exact name (breaking ties by depth and
	// then by presence of an explicitly provided JSON name).
	// Select the dominant field from each set of fields with the same name.
	// If multiple fields have the same name, then the dominant field
	// is the one that exists alone at the shallowest depth,
	// or the one that is uniquely tagged with a JSON name.
	// Otherwise, no dominant field exists for the set.
	flattened := allFields[:0]
	slices.SortFunc(allFields, func(x, y structField) int {
		switch {
		case x.Name != y.Name:
			return strings.Compare(x.Name, y.Name)
		case len(x.index) != len(y.index):
			return cmp.Compare(len(x.index), len(y.index))
		case x.HasName && !y.HasName:
			return -1
		case !x.HasName && y.HasName:
			return +1
		default:
			return 0
		}
	})
	for len(allFields) > 0 {
		n := 1 // number of fields with the same exact name
		for n < len(allFields) && allFields[n-1].Name == allFields[n].Name {
			n++
		}
		if n == 1 || len(allFields[0].index) != len(allFields[1].index) || allFields[0].HasName != allFields[1].HasName {
			flattened = append(flattened, allFields[0]) // only keep field if there is a dominant field
		}
		allFields = allFields[n:]
	}
	// Sort the fields according to a breadth-first ordering
	// so that we can re-number IDs with the smallest possible values.
	// This optimizes use of uintSet such that it fits in the 64-entry bit set.
	slices.SortFunc(flattened, func(x, y structField) int {
		return cmp.Compare(x.id, y.id)
	})
	for i := range flattened {
		flattened[i].id = i
	}

	// Sort the fields according to a depth-first ordering
	// as the typical order that fields are marshaled.
	slices.SortFunc(flattened, func(x, y structField) int {
		return slices.Compare(x.index, y.index)
	})
	// Compute the mapping of fields in the byActualName map.
	// Pre-fold all names so that we can lookup folded names quickly.
	fs := structFields{
		flattened:    flattened,
		byActualName: make(map[string]*structField, len(flattened)),
	}
	for i, f := range fs.flattened {
		fs.byActualName[f.Name] = &fs.flattened[i]
	}

	return fs, nil
}

type fieldOptions struct {
	Name      string
	HasName   bool
	AsSet     bool
	Inline    bool
	OmitZero  bool //omit zero value items
	OmitEmpty bool //omit empty arrays/maps/
	Mutable   bool
}

// parseFieldOptions parses the dynamo  tag in a Go struct field
func parseFieldOptions(sf reflect.StructField, tag string) (out fieldOptions, ignored bool, err error) {
	tag, hasTag := sf.Tag.Lookup(tag)

	// Check whether this field is explicitly ignored.
	if tag == "-" {
		return fieldOptions{}, true, nil
	}

	// Check whether this field is unexported.
	if !sf.IsExported() {
		// See https://go.dev/issue/21357 and https://go.de v/issue/24153.
		if sf.Anonymous {
			return out, ignored, fmt.Errorf("embedded Go struct field %s of an unexported type must be explicitly ignored with a `dynamo:\"-\"` tag", sf.Name)
		}
		// Tag options specified on an unexported field suggests user error.
		if hasTag {
			return out, ignored, fmt.Errorf("unexported Go struct field %s cannot have non-ignored `dynamo:%q` tag , either remove the tag or put it equal to \"-\"", sf.Name, tag)
		}
		// doesnt have tag
		//
		ignored = true
		return out, ignored, nil
	}

	// // Determine the member name for this Go field
	out.Name = sf.Name

	if len(tag) > 0 && !strings.HasPrefix(tag, ",") {
		var name string
		name, tag, _ = strings.Cut(tag, ",")
		if name != "" {
			out.HasName = true
			out.Name = name
		}
	}

	// fill the parameters for this value
	seenOpts := map[string]struct{}{}
	for len(tag) > 0 {
		var opt string
		opt, tag, _ = strings.Cut(tag, ",")
		switch opt {
		case "set":
			out.AsSet = true
		case "inline":
			out.Inline = true
		case "omitzero":
			out.OmitZero = true
		case "omitempty":
			out.OmitEmpty = true
		case "mutable":
			out.Mutable = true
		default:
			// Reject keys that resemble one of the supported options.
			normOpt := strings.ReplaceAll(strings.ToLower(opt), "_", "")
			switch normOpt {
			case "set", "inline", "omitzero", "omitempty", "mutable":
				return fieldOptions{}, ignored, fmt.Errorf("struct field %s has invalid appearance of `%s` tag option; specify `%s` instead", sf.Name, opt, normOpt)
			}
		}
		if _, ok := seenOpts[opt]; !ok {
			seenOpts[opt] = struct{}{}
		} else {
			return fieldOptions{}, ignored, fmt.Errorf("struct field %s has duplicate occurrence of option %s", sf.Name, opt)
		}
	}
	return out, false, err
}

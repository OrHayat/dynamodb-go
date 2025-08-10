package serializer

import (
	"encoding"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
)

var anyType = reflect.TypeFor[any]()
var boolType = reflect.TypeFor[bool]()
var float64Type = reflect.TypeFor[float64]()
var stringType = reflect.TypeFor[string]()
var byteSliceType = reflect.TypeFor[[]byte]()
var byteSliceSliceType = reflect.TypeFor[[][]byte]()
var sliceAnyType = reflect.TypeFor[[]any]()
var mapStringAnyType = reflect.TypeFor[map[string]any]()
var stringSliceType = reflect.TypeFor[[]string]()

// dynamodb sdk attributevalue.Marshaler/Unmarshaler
var attributeValueMarshalerType = reflect.TypeFor[attributevalue.Marshaler]()
var attributeValueUnmarshalerType = reflect.TypeFor[attributevalue.Unmarshaler]()

// encoding.Text.Marshaler/Unmarshaler
var textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

// encoding.Binary.Marshaler/Unmarshaler
var binaryMarshalerType = reflect.TypeFor[encoding.BinaryMarshaler]()
var binaryUnMarshalerType = reflect.TypeFor[encoding.BinaryUnmarshaler]()

// newAddressableValue constructs a new addressable value of type t.
func newAddressableValue(t reflect.Type) addressableValue {
	return addressableValue{reflect.New(t).Elem()}
}

// type for marking values in golang that can be addressed
type addressableValue struct {
	reflect.Value
}

func (va addressableValue) fieldByIndex(index []int, mayAlloc bool) addressableValue {
	for _, i := range index {
		va = va.indirect(mayAlloc)
		if !va.IsValid() {
			return va
		}
		va = addressableValue{va.Field(i)} // addressable if struct value is addressable
	}
	return va
}

func (va addressableValue) indirect(mayAlloc bool) addressableValue {
	if va.Kind() == reflect.Pointer {
		if va.IsNil() {
			if !mayAlloc {
				return addressableValue{}
			}
			va.Set(reflect.New(va.Type().Elem()))
		}
		va = addressableValue{va.Elem()} // dereferenced pointer is always addressable
	}
	return va
}

// implementsWhich is like t.Implements(ifaceType) for a list of interfaces,
// but checks whether either t or reflect.PointerTo(t) implements the interface.
func implementsWhich(t reflect.Type, ifaceTypes ...reflect.Type) (which reflect.Type, needAddr bool) {
	for _, ifaceType := range ifaceTypes {
		if needAddr, ok := implements(t, ifaceType); ok {
			return ifaceType, needAddr
		}
	}
	return nil, false
}

// implements is like t.Implements(ifaceType) but checks whether
// either t or reflect.PointerTo(t) implements the interface.
// It also reports whether the value needs to be addressed
// in order to satisfy the interface.
func implements(t, ifaceType reflect.Type) (needAddr, ok bool) {
	switch {
	case t.Implements(ifaceType):
		return false, true
	case reflect.PointerTo(t).Implements(ifaceType):
		return true, true
	default:
		return false, false
	}
}

// isAnyType reports wether t is equivalent to the any interface type.
func isAnyType(t reflect.Type) bool {
	// This is forward compatible if the Go language permits type sets within
	// ordinary interfaces where an interface with zero methods does not
	// necessarily mean it can hold every possible Go type.
	// See https://go.dev/issue/45346.
	return t == anyType || anyType.Implements(t)
}

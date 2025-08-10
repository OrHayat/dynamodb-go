package serializer

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var serializerCache sync.Map = sync.Map{}

func lookupSerializer(t reflect.Type) *serializer {
	val, ok := serializerCache.Load(t)
	if ok {
		return val.(*serializer)
	}
	val = newSerializerForType(t)
	val, _ = serializerCache.LoadOrStore(t, val)
	return val.(*serializer)
}

type serializer struct {
	marshal    marshalFn
	unmarshal  unmarshalFn
	nonDefault bool //if true the serializer or deserializer are not implemented in this package
}

func newSerializerForType(t reflect.Type) *serializer {
	serializer := makeDefaultSerializer(t)
	addCustomInterfaceSerializer(t, serializer)
	return serializer
}

func makeDefaultSerializer(t reflect.Type) *serializer {
	switch t.Kind() {
	case reflect.Bool:
		return makeBoolSerializer(t)
	case reflect.String:
		return makeStringSerializer(t)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return makeIntSerializer(t)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return makeUIntSerializer(t)
	case reflect.Float32, reflect.Float64:
		return makeFloatSerializer(t)
	case reflect.Slice:
		return makeSliceSerializer(t)
	case reflect.Array:
		return makeArraySerializer(t)
	case reflect.Map:
		return makeMapSerializer(t)
	case reflect.Pointer:
		return makePointerSerializer(t)
	case reflect.Struct:
		return makeStructSerializer(t)
	case reflect.Interface:
		return makeInterfaceSerializer(t)
	default:
		return makeNotSupportedSerializer(t)
	}
}

func makeBoolSerializer(t reflect.Type) *serializer {
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, state encoderState) (types.AttributeValue, error) {
			return &types.AttributeValueMemberBOOL{Value: av.Bool()}, nil
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberBOOL:
				va.SetBool(casted.Value)
			default:
				return NewUnSupportedTypeError(av, t)
			}
			return nil
		},
		nonDefault: false,
	}
}

func makeStringSerializer(t reflect.Type) *serializer {
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			return &types.AttributeValueMemberS{Value: av.String()}, nil
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberS:
				va.Value.SetString(casted.Value)
			default:
				return NewUnSupportedTypeError(av, t)
			}
			return nil
		},
		nonDefault: false,
	}
}

func makeIntSerializer(t reflect.Type) *serializer {
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			num := strconv.FormatInt(av.Int(), 10)
			return &types.AttributeValueMemberN{Value: num}, nil
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberN:
				var n int64
				n, err = strconv.ParseInt(casted.Value, 10, 64)
				if err != nil {
					return newUnMarshalError(
						SerializerErrorData{
							Kind:   DynamoKindNumber,
							GoType: t,
							Err:    err,
						},
					)
				}
				va.SetInt(n)
				return nil
			}
			return NewUnSupportedTypeError(av, t)
		},
		nonDefault: false,
	}
}

func makeUIntSerializer(t reflect.Type) *serializer {
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			num := strconv.FormatUint(av.Uint(), 10)
			return &types.AttributeValueMemberN{Value: num}, nil
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberN:
				var n uint64
				n, err = strconv.ParseUint(casted.Value, 10, 64)
				if err != nil {
					return newUnMarshalError(
						SerializerErrorData{
							Kind:   DynamoKindNumber,
							GoType: t,
							Err:    err,
						},
					)
				}
				va.SetUint(n)
				return nil
			}
			return NewUnSupportedTypeError(av, t)
		},
		nonDefault: false,
	}
}

func makeFloatSerializer(t reflect.Type) *serializer {
	bits := t.Bits()
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			num := strconv.FormatFloat(av.Float(), 'g', -1, bits)
			return &types.AttributeValueMemberN{Value: num}, nil
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberN:
				var n float64
				n, err = strconv.ParseFloat(casted.Value, 64)
				if err != nil {
					return newUnMarshalError(
						SerializerErrorData{
							Kind:   DynamoKindNumber,
							GoType: t,
							Err:    err,
						},
					)
				}
				va.SetFloat(n)
				return nil
			}
			return NewUnSupportedTypeError(av, t)
		},
		nonDefault: false,
	}
}

func makeBytesSliceSerializer(t reflect.Type) *serializer {
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			return &types.AttributeValueMemberB{Value: av.Bytes()}, nil
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberNULL:
				if casted.Value {
					va.SetZero()
					return nil
				}
			case *types.AttributeValueMemberB:
				va.SetBytes(casted.Value)
				return nil
			}
			return NewUnSupportedTypeError(av, t)
		},
		nonDefault: false,
	}
}
func makeSliceSerializer(t reflect.Type) *serializer {

	elementType := t.Elem()
	elementTypeKind := elementType.Kind()
	if elementTypeKind == reflect.Uint8 { //[]byte or []<byte_alias>
		return makeBytesSliceSerializer(t)
	}
	var elementsSerializer *serializer
	initElementSerializer := sync.Once{} //don't call init callback more then once
	initElementSerializerFn := func() {  //call this function only once to init elementsSerializer
		elementsSerializer = lookupSerializer(t.Elem())
	}
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			n := av.Len()

			if flags.AsSet {
				if elementType == byteSliceType { //[][]byte
					if n == 0 {
						return &types.AttributeValueMemberNULL{Value: true}, nil
					}
					res := make([][]byte, n)
					for i := range n {
						res[i] = av.Index(i).Bytes()
					}
					return &types.AttributeValueMemberBS{Value: res}, nil
				}
				switch elementTypeKind {
				case reflect.String: //[]string
					if n == 0 {
						return &types.AttributeValueMemberNULL{Value: true}, nil
					}
					res := make([]string, n)
					for i := range n {
						res[i] = av.Index(i).String()
					}
					return &types.AttributeValueMemberSS{Value: res}, nil
				case reflect.Float32, reflect.Float64:
					if n == 0 {
						return &types.AttributeValueMemberNULL{Value: true}, nil
					}
					res := make([]string, n)
					for i := range n {
						res[i] = strconv.FormatFloat(av.Index(i).Float(), 'f', -1, 64)
					}
					return &types.AttributeValueMemberNS{Value: res}, nil
				case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
					if n == 0 {
						return &types.AttributeValueMemberNULL{Value: true}, nil
					}
					res := make([]string, n)
					for i := range n {
						res[i] = strconv.FormatInt(av.Index(i).Int(), 10)
					}
					return &types.AttributeValueMemberNS{Value: res}, nil
				case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint, reflect.Uintptr:
					if n == 0 {
						return &types.AttributeValueMemberNULL{Value: true}, nil
					}
					res := make([]string, n)
					for i := range n {
						res[i] = strconv.FormatUint(av.Index(i).Uint(), 10)
					}
					return &types.AttributeValueMemberNS{Value: res}, nil
				}
				return nil, newMarshalError(
					SerializerErrorData{
						Kind:   DynamoKindUnknown,
						GoType: t,
						Err:    fmt.Errorf("cant marshal item as set:%w", errors.ErrUnsupported),
					},
				)
			}
			initElementSerializer.Do(initElementSerializerFn)
			list := make([]types.AttributeValue, 0, n)
			for i := range n {
				curItem := addressableValue{av.Index(i)} //slice[i] is addressable it safe to call &slice[i]
				encodedElement, err := elementsSerializer.marshal(e, curItem, flags)
				if err != nil {
					return nil, newMarshalError(
						SerializerErrorData{
							Kind:   DynamoKindList,
							GoType: t,
							Err:    err,
						},
					)
				}
				if encodedElement == nil {
					continue
				}
				list = append(list, encodedElement)
			}
			list = slices.Clip(list)
			return &types.AttributeValueMemberL{Value: list}, nil
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberNULL:
				if casted.Value {
					va.SetZero()
					return nil
				}
			case *types.AttributeValueMemberL:
				initElementSerializer.Do(initElementSerializerFn)
				n := len(casted.Value)
				slice := reflect.MakeSlice(t, n, n)
				for i := range n {
					err = elementsSerializer.unmarshal(decoder, casted.Value[i], addressableValue{slice.Index(i)}, flags)
					if err != nil {
						return newUnMarshalError(
							SerializerErrorData{
								Kind:   DynamoKindList,
								GoType: t,
								Err:    err,
							},
						)
					}
				}
				va.Set(slice)
				return nil
			}
			return NewUnSupportedTypeError(av, t)
		},
		nonDefault: false,
	}
}

var ErrUnMarshalArrayOverflow = errors.New("array overflow")
var ErrUnMarshalArrayUnderflow = errors.New("array underflow")

func makeBytesArraySerializer(t reflect.Type) *serializer {
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			return &types.AttributeValueMemberB{Value: av.Bytes()}, nil
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberNULL:
				if casted.Value {
					va.SetZero()
					return nil
				}
			case *types.AttributeValueMemberB:
				dst := va.Bytes()
				//copy src into dst
				numElementsToWrite := len(casted.Value)
				arrayLen := va.Len()
				if arrayLen < numElementsToWrite {
					if !decoder.config.unmarshalArrayBoundsCheck.IsOverflowAllowed() {
						return newUnMarshalError(
							SerializerErrorData{
								GoType: t,
								Kind:   DynamoKindList,
								Err:    ErrUnMarshalArrayOverflow,
							},
						)
					}
				}

				if arrayLen > numElementsToWrite {
					if !decoder.config.unmarshalArrayBoundsCheck.IsUnderflowAllowed() {
						return newUnMarshalError(
							SerializerErrorData{
								GoType: t,
								Kind:   DynamoKindList,
								Err:    ErrUnMarshalArrayUnderflow,
							},
						)
					}
				}
				clear(dst[copy(dst, casted.Value):]) // noop if len(b) <= len(dst) will zero all elements that are after the copied data if they exists
				return nil
			}
			return NewUnSupportedTypeError(av, t)
		},
		nonDefault: false,
	}
}
func makeArraySerializer(t reflect.Type) *serializer {
	elementType := t.Elem()
	if elementType.Kind() == reflect.Uint8 { //[]byte or []<byte_alias>
		return makeBytesArraySerializer(t)
	}
	var elementsSerializer *serializer
	initElementSerializer := sync.Once{} //dont call init callback more then once
	initElementSerializerFn := func() {  //call this function only once to initalize elementsSerializer
		elementsSerializer = lookupSerializer(t.Elem())
	}
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			n := av.Len()
			initElementSerializer.Do(initElementSerializerFn)
			list := make([]types.AttributeValue, 0, n)
			for i := range n {
				curItem := addressableValue{av.Index(i)} //slice[i] is adressable it safe to call &slice[i]
				encodedElement, err := elementsSerializer.marshal(e, curItem, flags)
				if err != nil {
					return nil, newMarshalError(
						SerializerErrorData{
							Kind:   DynamoKindList,
							GoType: t,
							Err:    err,
						},
					)
				}
				if encodedElement == nil {
					continue
				}
				list = append(list, encodedElement)
			}
			list = slices.Clip(list)
			return &types.AttributeValueMemberL{Value: list}, nil
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberNULL:
				if casted.Value {
					va.SetZero()
					return nil
				}
			case *types.AttributeValueMemberL:
				initElementSerializer.Do(initElementSerializerFn)
				numElementsToWrite := len(casted.Value)
				arrayLen := va.Len()
				if arrayLen < numElementsToWrite {
					if !decoder.config.unmarshalArrayBoundsCheck.IsOverflowAllowed() {
						return newUnMarshalError(
							SerializerErrorData{
								GoType: t,
								Kind:   DynamoKindList,
								Err:    ErrUnMarshalArrayOverflow,
							},
						)
					}
				}
				if arrayLen > numElementsToWrite {
					if !decoder.config.unmarshalArrayBoundsCheck.IsUnderflowAllowed() {
						return newUnMarshalError(
							SerializerErrorData{
								GoType: t,
								Kind:   DynamoKindList,
								Err:    ErrUnMarshalArrayUnderflow,
							},
						)
					}
				}
				for i := range min(arrayLen, numElementsToWrite) {
					v := addressableValue{va.Index(i)} // indexed array element is addressable if array is addressable
					err = elementsSerializer.unmarshal(decoder, casted.Value[i], v, flags)
					if err != nil {
						return newUnMarshalError(
							SerializerErrorData{
								GoType: t,
								Err:    err,
								Kind:   DynamoKindList,
							},
						)
					}
				}
				for i := numElementsToWrite; i < arrayLen; i++ {
					va.Index(i).SetZero()
				}
				return nil
			}
			return NewUnSupportedTypeError(av, t)
		},
		nonDefault: false,
	}
}

func makeNotSupportedSerializer(t reflect.Type) *serializer {
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			return nil, newMarshalError(
				SerializerErrorData{
					Kind:   DynamoKindUnknown,
					GoType: t,
					Err:    errors.ErrUnsupported,
				})
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			return newUnMarshalError(
				SerializerErrorData{
					Kind:   DynamoKindUnknown,
					GoType: t,
					Err:    errors.ErrUnsupported,
				},
			)
		},
		nonDefault: false,
	}
}

func makeStructSerializer(t reflect.Type) *serializer {

	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			if res, err, ok := e.config.marshalersRegistry.marshalFromCustomRegistry(t, e, av, flags); ok { //.unmarshalFromCustomRegistry(t, e, av, va, flags); ok {
				return res, err
			}
			sf, initStructFieldsErr := makeStructFieldsCached(t, e.config.tag)
			if initStructFieldsErr != nil {
				err := initStructFieldsErr
				err.Kind = DynamoKindMap
				err.action = "marshal"
				return nil, err
			}

			res := map[string]types.AttributeValue{}
			for i := range sf.flattened {
				f := &sf.flattened[i]
				//for example &x.y is valid so this should be addressable
				v := addressableValue{av.Field(f.index[0])} // addressable if struct value is addressable,
				if len(f.index) > 1 {
					v = v.fieldByIndex(f.index[1:], false)
					if !v.IsValid() {
						continue // implies a nil inlined field
					}
				}
				curFlags := flags
				curFlags.AsSet = f.AsSet

				// OmitZero skips the field if the Go value is zero,
				// which can be checked up front without calling the marshaler.
				if (e.config.omitZeroStructFields || f.OmitZero) && ((f.isZero == nil && v.IsZero()) || (f.isZero != nil && f.isZero(v))) {
					continue
				}
				nonDefault := f.serializersFunctions.nonDefault
				//  fast case for omit empty :
				//   if omit empty is enabled on this field
				// + if the field has registered callback for this type that return that the value is empty value for the type
				// + is the field is marshaled from this package (non default is false)
				if f.OmitEmpty && f.isEmpty != nil && f.isEmpty(v) && !nonDefault {
					continue // fast path for omitempty
				}
				marshal := f.serializersFunctions.marshal
				tmp, err := marshal(e, v, curFlags)
				if err != nil {
					return nil, err
				}
				if tmp == nil {
					continue
				}
				//slow case for omit empty - type cast and check is the value is empty value
				if f.OmitEmpty {
					switch e := tmp.(type) {
					case *types.AttributeValueMemberNULL:
						if e.Value {
							continue
						}
					case *types.AttributeValueMemberS:
						if e.Value == "" {
							continue
						}
					case *types.AttributeValueMemberM:
						if len(e.Value) == 0 {
							continue
						}
					case *types.AttributeValueMemberL:
						if len(e.Value) == 0 {
							continue
						}
					case *types.AttributeValueMemberB:
						if len(e.Value) == 0 {
							continue
						}
					}
				}
				res[f.Name] = tmp
			}
			return &types.AttributeValueMemberM{Value: res}, nil

		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			if err, ok := decoder.config.unmarshalersRegistry.unmarshalFromCustomRegistry(t, decoder, av, va, flags); ok {
				return err
			}
			switch casted := av.(type) {
			case *types.AttributeValueMemberNULL:
				if casted.Value {
					va.SetZero()
					return nil
				}
			case *types.AttributeValueMemberM:
				sf, initStructFieldsErr := makeStructFieldsCached(t, decoder.config.tag)
				if initStructFieldsErr != nil {
					err := initStructFieldsErr
					err.Kind = DynamoKindMap
					err.action = "unmarshal"
					return err
				}

				for key, curAv := range casted.Value {
					field, ok := sf.byActualName[key]
					if !ok {
						continue
					}
					// field.index[0]
					// curVal := addressableValue{va.Field(field.index[0])}
					curVal := addressableValue{va.Field(field.index[0])} // addressable if struct value is addressable
					if len(field.index) > 1 {
						//handle embedded structs by traversing the indexes
						curVal = curVal.fieldByIndex(field.index[1:], true)
						if !curVal.IsValid() {
							return newUnMarshalError(
								SerializerErrorData{
									GoType: t,
									Kind:   DynamoKindMap,
									Err:    fmt.Errorf("cannot set embedded pointer to unexported struct type"),
								},
							)
						}
					}
					err = field.serializersFunctions.unmarshal(decoder, curAv, curVal, flags)
					if err != nil {
						return newUnMarshalError(
							SerializerErrorData{
								Kind:   DynamoKindMap,
								GoType: t,
								Err:    err,
							},
						)
					}
				}
				return nil
			}
			return NewUnSupportedTypeError(av, t)
		},
		nonDefault: false,
	}
}

func makeInterfaceSerializer(t reflect.Type) *serializer {
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			if av.IsNil() {
				return &types.AttributeValueMemberNULL{Value: true}, nil
			}
			if t == anyType {
				return marshalAnyValue(e, av.Elem().Interface(), flags)
			}
			typ := av.Elem().Type()
			v := newAddressableValue(typ)
			v.Set(av.Elem())
			marshaler := lookupSerializer(typ)
			return marshaler.marshal(e, v, flags)
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberNULL:
				if casted.Value {
					va.SetZero()
					return nil
				}
			}
			var v addressableValue
			if va.IsNil() {
				if t == anyType {
					v, err := unmarshalAnyValue(decoder, av, flags)
					// We must check for nil interface values up front.
					// See https://go.dev/issue/52310.
					if v != nil {
						va.Set(reflect.ValueOf(v))
					}
					return err
				}
				if !isAnyType(t) {
					return fmt.Errorf("cannot unmarshal type %s input value is nil and interface is not any or any alias", t.String())
				}
				switch av.(type) {
				case *types.AttributeValueMemberB:
					v = newAddressableValue(byteSliceType)
				case *types.AttributeValueMemberBOOL:
					v = newAddressableValue(boolType)
				case *types.AttributeValueMemberN:
					v = newAddressableValue(float64Type)
				case *types.AttributeValueMemberS:
					v = newAddressableValue(stringType)
				case *types.AttributeValueMemberBS:
					v = newAddressableValue(byteSliceSliceType)
				case *types.AttributeValueMemberNS:
					v = newAddressableValue(float64Type)
				case *types.AttributeValueMemberSS:
					v = newAddressableValue(stringSliceType)
				case *types.AttributeValueMemberL:
					v = newAddressableValue(sliceAnyType)
				case *types.AttributeValueMemberM:
					v = newAddressableValue(mapStringAnyType)
				default:
					return NewUnSupportedTypeError(av, t)
				}
			} else {
				v = newAddressableValue(va.Elem().Type())
				v.Set(va.Elem())
			}
			unmarshal := lookupSerializer(v.Type()).unmarshal
			err = unmarshal(decoder, av, v, flags)
			va.Set(v.Value)
			return err
		},
	}
}

func makeMapSerializer(t reflect.Type) *serializer {
	var keySerializer *mapKeySerializer
	var valSerializer *serializer
	var keyType reflect.Type = t.Key()
	var valType reflect.Type = t.Elem()
	initMapSerializerFn := func() {
		keySerializer = lookupMapKeySerializer(keyType)
		valSerializer = lookupSerializer(valType)
	}

	initMapSerializer := sync.Once{}
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			//TODO : support map[string]strict{} ,support map[string]bool ,support map[<number>]struct{} to set

			initMapSerializer.Do(initMapSerializerFn)
			//map elements are not addressable without copying them to other variable manually
			// for example x:= map[string]string{"x":"abc"}
			//			   y:=&x["x"] is NOT valid go expression but
			//  		   z:=x["x"]
			//    		   y:=&z is valid go expression
			//so pre allocate 2 empty values 1 for key and one for value
			//copy  the map elements into them during iteration to get addressable values
			//
			k := newAddressableValue(keyType)
			v := newAddressableValue(valType)
			res := map[string]types.AttributeValue{}
			for iter := av.Value.MapRange(); iter.Next(); {
				k.SetIterKey(iter)
				v.SetIterValue(iter)
				encodedKey, err := keySerializer.marshal(k)
				if err != nil {
					return nil, newMarshalError(
						SerializerErrorData{
							Kind:   DynamoKindMap,
							GoType: t,
							Err:    err,
						},
					)
				}
				//skip empty keys
				if encodedKey == "" {
					continue
				}
				//go map ensures that keys are uniques -check for duplicates keys only if custom marshaler is used
				if keySerializer.checkDuplicateKeys {
					_, ok := res[encodedKey]
					if ok {
						return nil, newMarshalError(SerializerErrorData{
							Kind:   DynamoKindMap,
							GoType: t,
							Err:    fmt.Errorf("marshal key encountered duplicate key with value %s", encodedKey),
						})
					}
				}
				encodedVal, err := valSerializer.marshal(e, v, flags)
				if err != nil {
					return nil, newMarshalError(
						SerializerErrorData{
							Kind:   DynamoKindMap,
							GoType: t,
							Err:    err,
						},
					)
				}
				if encodedVal == nil {
					continue
				}
				res[encodedKey] = encodedVal
			}
			return &types.AttributeValueMemberM{Value: res}, nil
		},
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberNULL:
				if casted.Value {
					va.SetZero()
					return nil
				}
			case *types.AttributeValueMemberM:
				// casted.Value
				initMapSerializer.Do(initMapSerializerFn)
				var m reflect.Value
				if va.IsNil() {
					m = reflect.MakeMapWithSize(t, len(casted.Value))
				} else {
					m = va.Value
				}
				k := newAddressableValue(keyType)
				v := newAddressableValue(valType)

				for key, val := range casted.Value {
					err = keySerializer.unmarshal(key, k)
					if err != nil {
						return newUnMarshalError(
							SerializerErrorData{
								Kind:   DynamoKindMap,
								GoType: t,
								Err:    err,
							},
						)
					}
					err = valSerializer.unmarshal(decoder, val, v, flags)
					if err != nil {
						return newUnMarshalError(
							SerializerErrorData{
								Kind:   DynamoKindMap,
								GoType: t,
								Err:    err,
							},
						)
					}
					m.SetMapIndex(k.Value, v.Value)
				}
				va.Set(m)
				return nil
			}
			return NewUnSupportedTypeError(av, t)
		},
		nonDefault: false,
	}
}

func makePointerSerializer(t reflect.Type) *serializer {
	var elementsSerializer *serializer
	initElementSerializer := sync.Once{} //don't call init callback more then once
	initElementSerializerFn := func() {  //call this function only once to init elementsSerializer
		elementsSerializer = lookupSerializer(t.Elem())
	}
	return &serializer{
		marshal: func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			if av.IsNil() {
				return &types.AttributeValueMemberNULL{Value: true}, nil
			}
			initElementSerializer.Do(initElementSerializerFn)
			av = addressableValue{Value: av.Elem()} //pointer dereference is addressable in go
			//var x int
			//&*x is valid go expression
			return elementsSerializer.marshal(e, av, flags)
		},
		// fncs.unmarshal = func(dec *jsontext.Decoder, va addressableValue, uo *jsonopts.Struct) error {
		unmarshal: func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberNULL:
				if casted.Value {
					va.SetZero()
					return nil
				}
			}
			initElementSerializer.Do(initElementSerializerFn)
			if va.IsNil() {
				va.Set(reflect.New(t.Elem()))
			}
			v := addressableValue{va.Elem()}
			err = elementsSerializer.unmarshal(decoder, av, v, flags)
			return err
		},
		nonDefault: false,
	}
}

type mapKeySerializer struct {
	marshal            func(addressableValue) (string, error)
	unmarshal          func(string, addressableValue) (err error)
	checkDuplicateKeys bool //if true check duplicate for duplicate keys manually otherwise assume that the keys are already unique in the source map so skip the check
}

var mapKeySerializerCache sync.Map = sync.Map{} //*xsync.MapOf[reflect.Type, *mapKeySerializer] = xsync.NewMapOf[reflect.Type, *mapKeySerializer]()

func lookupMapKeySerializer(t reflect.Type) *mapKeySerializer {
	val, ok := mapKeySerializerCache.Load(t)
	if ok {
		return val.(*mapKeySerializer)
	}
	val = newMapKeySerializerForType(t)
	val, _ = mapKeySerializerCache.LoadOrStore(t, val)
	return val.(*mapKeySerializer)
}

func newMapKeySerializerForType(t reflect.Type) *mapKeySerializer {
	s := makeDefaultMapKeySerializer(t)
	addCustomInterfaceMapKeySerializer(t, s)
	return s
}

func makeDefaultMapKeySerializer(t reflect.Type) *mapKeySerializer {
	switch t.Kind() {
	case reflect.Bool:
		return &mapKeySerializer{
			marshal: func(av addressableValue) (string, error) {
				return strconv.FormatBool(av.Bool()), nil
			},
			unmarshal: func(s string, av addressableValue) (err error) {
				b, err := strconv.ParseBool(s)
				if err != nil {
					return newUnMarshalError(
						SerializerErrorData{
							Err:    err,
							GoType: t,
							Kind:   DynamoKindBool,
						},
					)
				}
				av.SetBool(b)
				return nil
			},
			checkDuplicateKeys: false,
		}
	case reflect.String:
		return &mapKeySerializer{
			marshal: func(av addressableValue) (string, error) {
				return av.String(), nil
			},
			unmarshal: func(s string, av addressableValue) (err error) {
				av.SetString(s)
				return nil
			},
			checkDuplicateKeys: false,
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &mapKeySerializer{
			marshal: func(av addressableValue) (string, error) {
				return strconv.FormatInt(av.Int(), 10), nil
			},
			unmarshal: func(s string, av addressableValue) (err error) {
				n, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return newUnMarshalError(
						SerializerErrorData{
							Err:    err,
							GoType: t,
							Kind:   DynamoKindNumber,
						},
					)
				}
				av.SetInt(n)
				return nil
			},
			checkDuplicateKeys: false,
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &mapKeySerializer{
			marshal: func(av addressableValue) (string, error) {
				return strconv.FormatUint(av.Uint(), 10), nil
			},
			unmarshal: func(s string, av addressableValue) (err error) {
				n, err := strconv.ParseUint(s, 10, 64)
				if err != nil {
					return newUnMarshalError(
						SerializerErrorData{
							Err:    err,
							GoType: t,
							Kind:   DynamoKindNumber,
						},
					)
				}
				av.SetUint(n)
				return nil
			},
			checkDuplicateKeys: false,
		}
	//float is not supported..
	default:
		return &mapKeySerializer{
			marshal: func(av addressableValue) (string, error) {
				return "", fmt.Errorf("map key of type %s is not supported", t.String())
			},
			unmarshal: func(s string, av addressableValue) (err error) {
				return fmt.Errorf("map key of type %s is not supported", t.String())
			},
			checkDuplicateKeys: false,
		}
	}
}

func addCustomInterfaceMapKeySerializer(t reflect.Type, serializer *mapKeySerializer) {
	//add custom serializer if needed
	marshaler, marshalerNeedAddr := implementsWhich(t, textMarshalerType)
	switch marshaler {
	case textMarshalerType:
		serializer.marshal = func(av addressableValue) (string, error) {
			var text []byte
			var err error
			if marshalerNeedAddr {
				text, err = av.Addr().Interface().(encoding.TextMarshaler).MarshalText()
			} else {
				text, err = av.Interface().(encoding.TextMarshaler).MarshalText()
			}
			if err != nil {
				return "", newMarshalError(
					SerializerErrorData{
						Kind:   DynamoKindUnknown,
						GoType: t,
						Err:    err,
					})
			}
			return string(text), nil
		}
		serializer.checkDuplicateKeys = true
	}

	unmarshaler, _ := implementsWhich(t, textUnmarshalerType)
	switch unmarshaler {
	case textUnmarshalerType:
		serializer.unmarshal = func(s string, av addressableValue) (err error) {
			err = av.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s)) //unmarshal has to be implemented on pointer value
			if err != nil {
				return newUnMarshalError(
					SerializerErrorData{
						Kind:   DynamoKindUnknown,
						GoType: t,
						Err:    err,
					},
				)
			}
			return nil
		}
	}
}

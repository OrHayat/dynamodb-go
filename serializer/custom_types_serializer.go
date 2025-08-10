package serializer

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type marshalFn func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error)

type customEncoderRegistry struct {
	marshalers sync.Map
}

func (r *customEncoderRegistry) addMarshaler(m customMarshaler) {

	r.marshalers.Store(m.typ, m.cb)
}

func (r *customEncoderRegistry) marshalFromCustomRegistry(t reflect.Type, e *Encoder, av addressableValue, flags encoderState) (val types.AttributeValue, err error, ok bool) {
	if r == nil {
		return nil, nil, false
	}
	fn, ok := r.marshalers.Load(t)
	if !ok {
		return nil, nil, false
	}
	val, err = fn.(marshalFn)(e, av, flags)
	return val, err, true
}

type customMarshaler struct {
	typ reflect.Type
	cb  marshalFn
}

func MarshalFuncV1[T any](fn func(T) (types.AttributeValue, error)) (res customMarshaler) {
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		panic(fmt.Errorf("cannot customize the marshaling of type %s only structs are supported", t.String()))
	}
	res.typ = t
	res.cb = func(e *Encoder, va addressableValue, flags encoderState) (types.AttributeValue, error) {
		av, err := fn(va.Value.Interface().(T)) //this convert the value to the supported value
		if err != nil {
			return av, newMarshalError(SerializerErrorData{
				GoType: t,
				Kind:   DynamoKindUnknown,
				Err:    err,
			})
		}
		return av, nil
	}

	return res
}

type unmarshalFn func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error)

type customDeserializerRegistry struct {
	unmarshalers sync.Map
}

func (r *customDeserializerRegistry) unmarshalFromCustomRegistry(t reflect.Type, decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error, ok bool) {
	if r == nil {
		return nil, false
	}
	rawCb, ok := r.unmarshalers.Load(t)
	if !ok {
		return nil, false
	}
	err = rawCb.(unmarshalFn)(decoder, av, va, flags)
	return err, true
}

func (r *customDeserializerRegistry) addUnmarshaler(m customUnMarshaler) {
	r.unmarshalers.Store(m.typ, m.cb)
}

type customUnMarshaler struct {
	typ reflect.Type
	cb  unmarshalFn
}

func UnMarshalFuncV1[T any](fn func(types.AttributeValue, *T) error) (res customUnMarshaler) {
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		panic(fmt.Errorf("cannot customize the marshaling of type %s only structs are supported", t.String()))
	}
	res.typ = t
	res.cb = func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
		err = fn(av, va.Value.Addr().Interface().(*T))
		if err != nil {
			return newUnMarshalError(
				SerializerErrorData{
					GoType: t,
					Kind:   DynamoKindUnknown,
					Err:    err,
				})
		}
		return nil
	}
	return res
}

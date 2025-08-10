package serializer

import (
	"encoding"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func addCustomInterfaceSerializer(t reflect.Type, serializer *serializer) *serializer {
	// Avoid injecting method marshaler on the pointer or interface version
	// to avoid ever calling the method on a nil pointer or interface receiver.
	// Let it be injected on the value receiver (which is always addressable).
	if t.Kind() == reflect.Pointer || t.Kind() == reflect.Interface {
		return serializer
	}
	//handle custom serializers

	marshaler, marshalerNeedAddr := implementsWhich(t, attributeValueMarshalerType, textMarshalerType, binaryMarshalerType)
	switch marshaler {
	case attributeValueMarshalerType:
		serializer.nonDefault = true
		serializer.marshal = func(e *Encoder, av addressableValue, flags encoderState) (res types.AttributeValue, err error) {
			if marshalerNeedAddr {
				res, err = av.Addr().Interface().(attributevalue.Marshaler).MarshalDynamoDBAttributeValue()
			} else {
				res, err = av.Interface().(attributevalue.Marshaler).MarshalDynamoDBAttributeValue()
			}
			if err != nil {
				return nil, newMarshalError(
					SerializerErrorData{
						Kind:   DynamoKindUnknown,
						GoType: t,
						Err:    err,
					},
				)
			}
			return res, err
		}
	case textMarshalerType:
		serializer.nonDefault = true
		serializer.marshal = func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			var text []byte
			var err error
			if marshalerNeedAddr {
				text, err = av.Addr().Interface().(encoding.TextMarshaler).MarshalText()
			} else {
				text, err = av.Interface().(encoding.TextMarshaler).MarshalText()
			}
			if err != nil {
				return nil, newMarshalError(
					SerializerErrorData{
						Kind:   DynamoKindUnknown,
						GoType: t,
						Err:    err,
					},
				)
			}
			if text == nil {
				return &types.AttributeValueMemberNULL{Value: true}, nil
			}
			return &types.AttributeValueMemberS{Value: string(text)}, nil
		}
	case binaryMarshalerType:
		serializer.nonDefault = true
		serializer.marshal = func(e *Encoder, av addressableValue, flags encoderState) (types.AttributeValue, error) {
			var data []byte
			var err error
			if marshalerNeedAddr {
				data, err = av.Addr().Interface().(encoding.BinaryMarshaler).MarshalBinary()
			} else {
				data, err = av.Interface().(encoding.BinaryMarshaler).MarshalBinary()
			}
			if err != nil {
				return nil, newMarshalError(
					SerializerErrorData{
						Kind:   DynamoKindUnknown,
						GoType: t,
						Err:    err,
					},
				)
			}
			return &types.AttributeValueMemberB{Value: data}, nil
		}
	}
	unmarshaler, unmarshalerNeedAddr := implementsWhich(t, attributeValueUnmarshalerType, textUnmarshalerType, binaryUnMarshalerType)
	switch unmarshaler {
	case attributeValueUnmarshalerType:
		serializer.unmarshal = func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			if unmarshalerNeedAddr {
				err = va.Addr().Interface().(attributevalue.Unmarshaler).UnmarshalDynamoDBAttributeValue(av)
			} else {
				err = va.Interface().(attributevalue.Unmarshaler).UnmarshalDynamoDBAttributeValue(av)
			}
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
	case textUnmarshalerType:
		serializer.unmarshal = func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberNULL:
				if casted.Value {
					va.SetZero()
					return
				}
			case *types.AttributeValueMemberS:
				data := []byte(casted.Value)
				if unmarshalerNeedAddr {
					err = va.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText(data)
				} else {
					err = va.Interface().(encoding.TextUnmarshaler).UnmarshalText(data)
				}
				if err != nil {
					return newUnMarshalError(
						SerializerErrorData{
							Kind:   dynamodbAvToKind(av),
							GoType: t,
							Err:    err,
						},
					)
				}
				return nil
			}
			return NewUnSupportedTypeError(av, t)
		}
	case binaryUnMarshalerType:
		serializer.unmarshal = func(decoder *Decoder, av types.AttributeValue, va addressableValue, flags decoderState) (err error) {
			switch casted := av.(type) {
			case *types.AttributeValueMemberNULL:
				if casted.Value {
					va.SetZero()
					return
				}
			case *types.AttributeValueMemberB:
				if unmarshalerNeedAddr {
					err = va.Addr().Interface().(encoding.BinaryUnmarshaler).UnmarshalBinary(casted.Value)
				} else {
					err = va.Interface().(encoding.BinaryUnmarshaler).UnmarshalBinary(casted.Value)
				}
				if err != nil {
					return newUnMarshalError(
						SerializerErrorData{
							Kind:   dynamodbAvToKind(av),
							GoType: t,
							Err:    err,
						},
					)
				}
				return nil
			}
			return NewUnSupportedTypeError(av, t)
		}

	}

	return serializer
}

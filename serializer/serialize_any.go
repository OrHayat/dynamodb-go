package serializer

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func marshalAnyValue(e *Encoder, val any, flags encoderState) (types.AttributeValue, error) {
	switch casted := val.(type) {
	case nil:
		return &types.AttributeValueMemberNULL{Value: true}, nil
	case bool:
		return &types.AttributeValueMemberBOOL{Value: casted}, nil
	case string:
		return &types.AttributeValueMemberS{Value: casted}, nil
	case int:
		num := strconv.FormatInt(int64(casted), 10)
		return &types.AttributeValueMemberN{Value: num}, nil
	case int8:
		num := strconv.FormatInt(int64(casted), 10)
		return &types.AttributeValueMemberN{Value: num}, nil
	case int16:
		num := strconv.FormatInt(int64(casted), 10)
		return &types.AttributeValueMemberN{Value: num}, nil
	case int32:
		num := strconv.FormatInt(int64(casted), 10)
		return &types.AttributeValueMemberN{Value: num}, nil
	case int64:
		num := strconv.FormatInt(int64(casted), 10)
		return &types.AttributeValueMemberN{Value: num}, nil
	case uint:
		num := strconv.FormatUint(uint64(casted), 10)
		return &types.AttributeValueMemberN{Value: num}, nil
	case uint8:
		num := strconv.FormatUint(uint64(casted), 10)
		return &types.AttributeValueMemberN{Value: num}, nil
	case uint16:
		num := strconv.FormatUint(uint64(casted), 10)
		return &types.AttributeValueMemberN{Value: num}, nil
	case uint32:
		num := strconv.FormatUint(uint64(casted), 10)
		return &types.AttributeValueMemberN{Value: num}, nil
	case uint64:
		num := strconv.FormatUint(uint64(casted), 10)
		return &types.AttributeValueMemberN{Value: num}, nil
	case float32:
		num := strconv.FormatFloat(float64(casted), 'g', -1, 32)
		return &types.AttributeValueMemberN{Value: num}, nil
	case float64:
		num := strconv.FormatFloat(float64(casted), 'g', -1, 64)
		return &types.AttributeValueMemberN{Value: num}, nil
	case map[string]any:
		return marshalObjectAny(e, casted, flags)
	case []any:
		return marshalArrayAny(e, casted, flags)
	default:
		v := newAddressableValue(reflect.TypeOf(val))
		v.Set(reflect.ValueOf(val))
		marshal := lookupSerializer(v.Type()).marshal
		return marshal(e, v, flags)
	}
}

func marshalObjectAny(e *Encoder, val map[string]any, flags encoderState) (types.AttributeValue, error) {
	res := map[string]types.AttributeValue{}
	for k, v := range val {
		encodedVal, err := marshalAnyValue(e, v, flags)
		if err != nil {
			return nil, newMarshalError(SerializerErrorData{
				Err:    err,
				GoType: mapStringAnyType,
				Kind:   DynamoKindMap,
			})
		}
		if encodedVal == nil {
			continue
		}
		res[k] = encodedVal
	}
	return &types.AttributeValueMemberM{Value: res}, nil
}

func marshalArrayAny(e *Encoder, val []any, flags encoderState) (types.AttributeValue, error) {
	res := make([]types.AttributeValue, 0, len(val))
	for _, v := range val {
		encodedVal, err := marshalAnyValue(e, v, flags)
		if err != nil {
			return nil, newMarshalError(SerializerErrorData{
				Err:    err,
				GoType: sliceAnyType,
				Kind:   DynamoKindMap,
			})
		}
		if encodedVal == nil {
			continue
		}
		res = append(res, encodedVal)
	}
	res = slices.Clip(res)
	return &types.AttributeValueMemberL{Value: res}, nil
}

func unmarshalAnyValue(decoder *Decoder, av types.AttributeValue, flags decoderState) (any, error) {
	switch casted := av.(type) {
	case *types.AttributeValueMemberNULL:
		if casted.Value {
			return nil, nil
		}
	case *types.AttributeValueMemberBOOL:
		return casted.Value, nil
	case *types.AttributeValueMemberS:
		return casted.Value, nil
	case *types.AttributeValueMemberN:
		fv, err := strconv.ParseFloat(casted.Value, 64)
		return fv, err
	case *types.AttributeValueMemberB:
		return casted.Value, nil
	case *types.AttributeValueMemberBS:
		return casted.Value, nil
	case *types.AttributeValueMemberSS:
		return casted.Value, nil
	case *types.AttributeValueMemberNS:
		numbers := make([]float64, len(casted.Value))
		for i, rawNumber := range casted.Value {
			n, err := strconv.ParseFloat(rawNumber, 64)
			if err != nil {
				return nil, err
			}
			numbers[i] = n
		}
		return numbers, nil
	case *types.AttributeValueMemberL:
		return unmarshalArrayAny(decoder, casted.Value, flags)
	case *types.AttributeValueMemberM:
		return unmarshalObjectAny(decoder, casted.Value, flags)
	}
	k := reflect.TypeOf(av)
	return nil, fmt.Errorf("BUG: invalid types.AttributevalueMember type:%s", k.String())

}

func unmarshalArrayAny(decoder *Decoder, items []types.AttributeValue, flags decoderState) ([]any, error) {
	if items == nil {
		return nil, nil
	}

	res := make([]any, len(items))
	for i, item := range items {
		curItem, err := unmarshalAnyValue(decoder, item, flags)
		if err != nil {
			return nil, err
		}
		res[i] = curItem
	}
	return res, nil
}

func unmarshalObjectAny(decoder *Decoder, items map[string]types.AttributeValue, flags decoderState) (res map[string]any, err error) {
	if items == nil {
		return nil, nil
	}

	res = make(map[string]any, len(items))
	for name, av := range items {
		val, err := unmarshalAnyValue(decoder, av, flags)
		if err != nil {
			return nil, err
		}
		res[name] = val
	}
	return res, nil
}

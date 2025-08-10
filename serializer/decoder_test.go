package serializer_test

import (
	"encoding"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/serializer"
	"github.com/stretchr/testify/require"
)

var _ encoding.TextUnmarshaler = encoding.TextUnmarshaler(nil)

type upperCaseTextUnmarshalerMock string

// UnmarshalText implements encoding.TextUnmarshaler.
func (u *upperCaseTextUnmarshalerMock) UnmarshalText(text []byte) error {
	*u = upperCaseTextUnmarshalerMock(strings.ToUpper(string(text)))
	return nil
}

var _ encoding.TextUnmarshaler = (*UnmarshalTextToNumber)(nil)

type UnmarshalTextToNumber struct {
	secret int64
}

// UnmarshalText UnmarshalTextToNumber encoding.TextUnmarshaler.
func (u *UnmarshalTextToNumber) UnmarshalText(text []byte) (err error) {
	if text == nil {
		return fmt.Errorf("got_nil")
	}
	u.secret, err = strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return fmt.Errorf("failed_to_parse_number")
	}
	return err
}

var _ encoding.BinaryUnmarshaler = (*binaryUnMarshalerMock)(nil)

type binaryUnMarshalerMock struct {
	Data      []byte
	ErrInject bool
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (b *binaryUnMarshalerMock) UnmarshalBinary(data []byte) error {
	if b.ErrInject {
		return fmt.Errorf("mock-error")
	}
	b.Data = data
	return nil
}

var _ attributevalue.Unmarshaler = (*attributeValueUnmarshalerMock)(nil)

type attributeValueUnmarshalerMock struct {
	ErrInject bool
	data      string
}

// UnmarshalDynamoDBAttributeValue implements attributevalue.Unmarshaler.
func (a *attributeValueUnmarshalerMock) UnmarshalDynamoDBAttributeValue(av types.AttributeValue) error {
	if a.ErrInject {
		return fmt.Errorf("mock-error")
	}
	sv, ok := av.(*types.AttributeValueMemberS)
	if ok {
		a.data = sv.Value
	} else {
		a.data = "ok"
	}
	return nil
}

type structA struct {
	A string
}

func TestDecoder(t *testing.T) {
	type decoderTestCase struct {
		name            string
		opts            []serializer.DecoderOption
		dynamoValue     types.AttributeValue
		unmarshalInput  any
		expectedGoValue any
		expectedError   error
	}
	cases := []decoderTestCase{
		{
			name:           "Decoder/InvalidInput/Nil",
			dynamoValue:    &types.AttributeValueMemberBS{Value: nil},
			unmarshalInput: nil,
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind: serializer.DynamoKindBinarySet,
					Err:  serializer.ErrInvalidUnmarshalInputType,
				},
			),
		},
		{
			name:           "Decoder/InvalidInput/NotPointer",
			dynamoValue:    &types.AttributeValueMemberS{Value: "a"},
			unmarshalInput: 40,
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind: serializer.DynamoKindString,
					Err:  serializer.ErrInvalidUnmarshalInputType,
				},
			),
		},
		{
			name:            "Bools/True",
			expectedGoValue: true,
			dynamoValue:     &types.AttributeValueMemberBOOL{Value: true},
			unmarshalInput:  new(bool),
		},
		{
			name:            "Bools",
			expectedGoValue: []bool{true, false},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberBOOL{Value: true},
					&types.AttributeValueMemberBOOL{Value: false},
				},
			},
			unmarshalInput: new([]bool),
			expectedError:  nil,
		},
		{
			name:            "Bools/Named",
			expectedGoValue: []namedBool{namedBool(true), namedBool(false), true},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberBOOL{Value: true},
					&types.AttributeValueMemberBOOL{Value: false},
					&types.AttributeValueMemberBOOL{Value: true}},
			},
			unmarshalInput: new([]namedBool),
			expectedError:  nil,
		},
		{
			name:           "Bools/InvalidType",
			dynamoValue:    &types.AttributeValueMemberS{Value: "abc"},
			unmarshalInput: new(bool),
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindString,
					GoType: reflect.TypeFor[bool](),
					Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberS"),
				},
			),
		},
		{
			name:            "Slices/Empty",
			expectedGoValue: []bool{},
			dynamoValue: &types.AttributeValueMemberL{
				Value: nil,
			},
			unmarshalInput: new([]bool),
			expectedError:  nil,
		},
		{
			name:            "Slices/Nil",
			expectedGoValue: ([]bool)(nil),
			dynamoValue: &types.AttributeValueMemberNULL{
				Value: true,
			},
			unmarshalInput: new([]bool),
			expectedError:  nil,
		},
		{
			name:            "Slices//Null#2",
			dynamoValue:     &types.AttributeValueMemberNULL{Value: true},
			expectedGoValue: []string(nil),
			unmarshalInput:  new([]string),
		},
		{
			name:           "Slice/UnsupportedElement",
			unmarshalInput: new([]string),
			dynamoValue:    &types.AttributeValueMemberS{Value: "a"},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindString,
					GoType: reflect.TypeFor[[]string](),
					Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberS"),
				}),
		},
		{
			name:           "Slices/IntSlice/Error",
			unmarshalInput: new([][]int),
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberN{Value: "not_number"},
						},
					},
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindList,
					GoType: reflect.TypeFor[[][]int](),
					Err: serializer.NewUnMarshalError(
						serializer.SerializerErrorData{
							Kind:   serializer.DynamoKindList,
							GoType: reflect.TypeFor[[]int](),
							Err: serializer.NewUnMarshalError(
								serializer.SerializerErrorData{
									Kind:   serializer.DynamoKindNumber,
									GoType: reflect.TypeFor[int](),
									Err:    fmt.Errorf("strconv.ParseInt: parsing \"not_number\": invalid syntax"),
								},
							),
						},
					),
				},
			),
		},
		{
			name:            "Arrays/Nil",
			dynamoValue:     &types.AttributeValueMemberNULL{Value: true},
			expectedGoValue: [2]string{},
			unmarshalInput:  new([2]string),
		},
		{
			name: "Arrays/String",
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "AA"},
					&types.AttributeValueMemberS{Value: "B"},
				},
			},
			expectedGoValue: [2]string{"AA", "B"},
			unmarshalInput:  new([2]string),
		},
		{
			name:            "Arrays/String/OverFlow/AllowUnmarshalArrayFromAnyLen",
			unmarshalInput:  new([5]string),
			opts:            []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.AllowUnmarshalArrayFromAnyLen)},
			expectedGoValue: [5]string{"1", "2", "a", "b", "c"},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "1"},
					&types.AttributeValueMemberS{Value: "2"},
					&types.AttributeValueMemberS{Value: "a"},
					&types.AttributeValueMemberS{Value: "b"},
					&types.AttributeValueMemberS{Value: "c"},
					&types.AttributeValueMemberS{Value: "d"}, //will be dropped from output
				},
			},
		},
		{
			name:            "Arrays/String/OverFlow/AllowUnmarshalArrayWithOverflow",
			expectedGoValue: [5]string{"1", "2", "a", "b", "c"},
			unmarshalInput:  new([5]string),
			opts:            []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckUnderflowArrayOnUnmarshaling)},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "1"},
					&types.AttributeValueMemberS{Value: "2"},
					&types.AttributeValueMemberS{Value: "a"},
					&types.AttributeValueMemberS{Value: "b"},
					&types.AttributeValueMemberS{Value: "c"},
					&types.AttributeValueMemberS{Value: "d"}, //will be dropped from output
					&types.AttributeValueMemberS{Value: "f"}, //will be dropped from output
				},
			},
		},
		{
			name:           "Arrays/String/OverFlow/AllowUnmarshalArrayWithUnderflow",
			unmarshalInput: new([5]string),
			opts:           []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckOverflowUnmarshalArrayOnUnmarshaling)},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "1"},
					&types.AttributeValueMemberS{Value: "2"},
					&types.AttributeValueMemberS{Value: "a"},
					&types.AttributeValueMemberS{Value: "b"},
					&types.AttributeValueMemberS{Value: "c"},
					&types.AttributeValueMemberS{Value: "d"}, //will be dropped from output
					&types.AttributeValueMemberS{Value: "f"}, //will be dropped from output
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindList,
					GoType: reflect.TypeFor[[5]string](),
					Err:    serializer.ErrUnMarshalArrayOverflow,
				},
			),
		},
		{
			name:           "Arrays/String/OverFlow/CheckOverFlowAndUnderflowOnUnmarshal",
			unmarshalInput: new([5]string),
			opts:           []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckOverFlowAndUnderflowArrayOnUnmarshal)},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "1"},
					&types.AttributeValueMemberS{Value: "2"},
					&types.AttributeValueMemberS{Value: "a"},
					&types.AttributeValueMemberS{Value: "b"},
					&types.AttributeValueMemberS{Value: "c"},
					&types.AttributeValueMemberS{Value: "d"}, //will be dropped from output
					&types.AttributeValueMemberS{Value: "f"}, //will be dropped from output
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindList,
					GoType: reflect.TypeFor[[5]string](),
					Err:    serializer.ErrUnMarshalArrayOverflow,
				},
			),
		},
		{
			name:            "Arrays/String/Underflow/AllowUnmarshalArrayFromAnyLen",
			opts:            []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.AllowUnmarshalArrayFromAnyLen)},
			unmarshalInput:  new([5]string),
			expectedGoValue: [5]string{"1", "2", "a", "", ""},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "1"},
					&types.AttributeValueMemberS{Value: "2"},
					&types.AttributeValueMemberS{Value: "a"},
				},
			},
		},
		{
			name:            "Arrays/Bool/Underflow/AllowUnmarshalArrayWithUnderflow",
			opts:            []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckOverflowUnmarshalArrayOnUnmarshaling)},
			unmarshalInput:  new([5]bool),
			expectedGoValue: [5]bool{true, false, true, false, false},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberBOOL{Value: true},
					&types.AttributeValueMemberBOOL{Value: false},
					&types.AttributeValueMemberBOOL{Value: true},
				},
			},
		},
		{
			name:            "Arrays/String/Underflow/AllowUnmarshalArrayWithOverflow",
			unmarshalInput:  new([5]string),
			opts:            []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckUnderflowArrayOnUnmarshaling)},
			expectedGoValue: [5]string{"1", "a"},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "1"},
					&types.AttributeValueMemberS{Value: "a"},
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindList,
					GoType: reflect.TypeFor[[5]string](),
					Err:    serializer.ErrUnMarshalArrayUnderflow,
				},
			),
		},
		{
			name:            "Arrays/String/OverFlow/CheckOverFlowAndUnderflowOnUnmarshal",
			unmarshalInput:  new([5]string),
			opts:            []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckUnderflowArrayOnUnmarshaling)},
			expectedGoValue: [5]string{"1", "a"},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "1"},
					&types.AttributeValueMemberS{Value: "a"},
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindList,
					GoType: reflect.TypeFor[[5]string](),
					Err:    serializer.ErrUnMarshalArrayUnderflow,
				},
			),
		},
		{
			name:           "Arrays/InvalidType",
			unmarshalInput: new([2]bool),
			dynamoValue: &types.AttributeValueMemberN{
				Value: "20",
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindNumber,
					GoType: reflect.TypeFor[[2]bool](),
					Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberN"),
				},
			),
		},
		{
			name:           "Arrays/InvalidElementType",
			unmarshalInput: new([2]bool),
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberNS{Value: []string{"1"}},
					&types.AttributeValueMemberS{Value: "a"},
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindList,
					GoType: reflect.TypeFor[[2]bool](),
					Err: serializer.NewUnMarshalError(serializer.SerializerErrorData{
						Kind:   serializer.DynamoKindNumberSet,
						GoType: reflect.TypeFor[bool](),
						Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberNS"),
					}),
				},
			),
		},
		{
			name:            "Strings",
			expectedGoValue: []string{"", "abc", "hello", "世界"},
			expectedError:   nil,
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: ""},
					&types.AttributeValueMemberS{Value: "abc"},
					&types.AttributeValueMemberS{Value: "hello"},
					&types.AttributeValueMemberS{Value: "世界"},
				},
			},
			unmarshalInput: new([]string),
		},
		{
			name:            "Strings/Named",
			expectedGoValue: []namedString{"", "abc", namedString("hello"), "世界"},
			expectedError:   nil,
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: ""},
					&types.AttributeValueMemberS{Value: "abc"},
					&types.AttributeValueMemberS{Value: "hello"},
					&types.AttributeValueMemberS{Value: "世界"},
				},
			},
			unmarshalInput: new([]namedString),
		},
		{
			name:           "Strings/InvalidType",
			dynamoValue:    &types.AttributeValueMemberBOOL{Value: true},
			unmarshalInput: new(string),
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindBool,
					GoType: reflect.TypeFor[string](),
					Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberBOOL"),
				},
			),
		},
		{
			name:            "Ints/Int",
			expectedGoValue: int(10),
			unmarshalInput:  new(int),
			dynamoValue:     &types.AttributeValueMemberN{Value: "10"},
		},
		{
			name:           "Ints/InvalidValue",
			unmarshalInput: new(int),
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[int](),
					Kind:   serializer.DynamoKindNumber,
					Err:    fmt.Errorf("strconv.ParseInt: parsing \"abc\": invalid syntax"),
				},
			),
			dynamoValue: &types.AttributeValueMemberN{Value: "abc"},
		},
		{
			name:           "Ints/InvalidType",
			unmarshalInput: new(int),
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[int](),
					Kind:   serializer.DynamoKindNumberSet,
					Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberNS"),
				},
			),
			dynamoValue: &types.AttributeValueMemberNS{Value: []string{}},
		},
		{
			name:            "UInts/Uint",
			expectedGoValue: uint(10),
			unmarshalInput:  new(uint),
			dynamoValue:     &types.AttributeValueMemberN{Value: "10"},
		},
		{
			name:           "UInts/InvalidValue",
			unmarshalInput: new(uint64),
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[uint64](),
					Kind:   serializer.DynamoKindNumber,
					Err:    fmt.Errorf("strconv.ParseUint: parsing \"-10\": invalid syntax"),
				},
			),
			dynamoValue: &types.AttributeValueMemberN{Value: "-10"},
		},
		{
			name:           "UInts/InvalidType",
			unmarshalInput: new(uint32),
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[uint32](),
					Kind:   serializer.DynamoKindNumberSet,
					Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberNS"),
				},
			),
			dynamoValue: &types.AttributeValueMemberNS{Value: []string{}},
		},
		{
			name:            "Floats/Float32",
			unmarshalInput:  new(float32),
			dynamoValue:     &types.AttributeValueMemberN{Value: "0.5"},
			expectedGoValue: float32(0.5),
		},
		{
			name:           "Floats/InvalidValue",
			unmarshalInput: new(float32),
			dynamoValue:    &types.AttributeValueMemberN{Value: "invalid"},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindNumber,
					GoType: reflect.TypeFor[float32](),
					Err:    fmt.Errorf("strconv.ParseFloat: parsing \"invalid\": invalid syntax"),
				},
			),
		},
		{
			name:           "Floats/InvalidType",
			unmarshalInput: new(float32),
			dynamoValue:    &types.AttributeValueMemberS{Value: "0.5"},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindString,
					GoType: reflect.TypeFor[float32](),
					Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberS"),
				},
			),
		},
		{
			name:            "Bytes",
			expectedGoValue: [][]byte{nil, {}, {1}, {1, 2}, {1, 2, 3}},
			unmarshalInput:  new([][]byte),
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberB{Value: nil},
					&types.AttributeValueMemberB{Value: []byte{}},
					&types.AttributeValueMemberB{Value: []byte{1}},
					&types.AttributeValueMemberB{Value: []byte{1, 2}},
					&types.AttributeValueMemberB{Value: []byte{1, 2, 3}},
				},
			},
		},
		{
			name:            "Bytes/Large",
			expectedGoValue: []byte("abcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()"),
			unmarshalInput:  new([]byte),
			dynamoValue: &types.AttributeValueMemberB{
				Value: []byte("abcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()"),
			},
		},
		{
			name:            "Bytes/Null",
			dynamoValue:     &types.AttributeValueMemberNULL{Value: true},
			expectedGoValue: []byte(nil),
			unmarshalInput:  new([]byte),
		},
		{
			name:           "Bytes/InvalidType",
			dynamoValue:    &types.AttributeValueMemberN{Value: "10"},
			unmarshalInput: addr(new([]byte)),
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindNumber,
					GoType: reflect.TypeFor[[]byte](),
					Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberN"),
				},
			),
		},
		{
			name:            "Bytes/Array/Null",
			dynamoValue:     &types.AttributeValueMemberNULL{Value: true},
			expectedGoValue: [2]byte{},
			unmarshalInput:  new([2]byte),
		},
		{
			name:            "Bytes/ByteArray",
			unmarshalInput:  new([5]byte),
			expectedGoValue: [5]byte{'h', 'e', 'l', 'l', 'o'},
			dynamoValue:     &types.AttributeValueMemberB{Value: []byte{'h', 'e', 'l', 'l', 'o'}},
		},
		{
			name:            "Bytes/ByteArray/OverFlow/AllowUnmarshalArrayFromAnyLen",
			unmarshalInput:  new([5]byte),
			expectedGoValue: [5]byte{'h', 'e', 'l', 'l', 0},
			opts:            []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.AllowUnmarshalArrayFromAnyLen)},
			dynamoValue: &types.AttributeValueMemberB{
				Value: []byte{'h', 'e', 'l', 'l'},
			},
		},
		{
			name:            "Bytes/ByteArray/OverFlow/AllowUnmarshalArrayWithOverflow",
			unmarshalInput:  new([6]byte),
			opts:            []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckUnderflowArrayOnUnmarshaling)},
			expectedGoValue: [6]byte{'h', 'e', 'l', 'l', 'o', '_'},
			dynamoValue: &types.AttributeValueMemberB{
				Value: []byte("hello_test12345"),
			},
		},
		{
			name:           "Bytes/ByteArray/OverFlow/CheckOverFlowAndUndeflowOnUnmarshal",
			unmarshalInput: new([5]byte),
			opts:           []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckOverFlowAndUnderflowArrayOnUnmarshal)},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindList,
					GoType: reflect.TypeFor[[5]byte](),
					Err:    serializer.ErrUnMarshalArrayUnderflow,
				},
			),
			dynamoValue: &types.AttributeValueMemberB{
				Value: []byte{'h', 'e', 0},
			},
		},
		{
			name:           "Bytes/ByteArray/OverFlow/AllowUnmarshalArrayWithUnderflow",
			unmarshalInput: new([5]byte),
			opts:           []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckOverflowUnmarshalArrayOnUnmarshaling)},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindList,
					GoType: reflect.TypeFor[[5]byte](),
					Err:    serializer.ErrUnMarshalArrayOverflow,
				},
			),
			dynamoValue: &types.AttributeValueMemberB{
				Value: []byte("hello_test12345"),
			},
		},
		{
			name:            "Bytes/ByteArray/UnderFlow/AllowUnmarshalArrayFromAnyLen",
			expectedGoValue: [5]byte{'h', 'e', 0, 0, 0},
			unmarshalInput:  new([5]byte),
			opts:            []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.AllowUnmarshalArrayFromAnyLen)},
			dynamoValue: &types.AttributeValueMemberB{
				Value: []byte{'h', 'e'},
			},
		},
		{
			name:            "Bytes/ByteArray/UnderFlow/AllowUnmarshalArrayWithUnderflow",
			expectedGoValue: [5]byte{'h', 'e', 'l'},
			unmarshalInput:  new([5]byte),
			opts:            []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckOverflowUnmarshalArrayOnUnmarshaling)},
			dynamoValue: &types.AttributeValueMemberB{
				Value: []byte{'h', 'e', 'l'},
			},
		},
		{
			name:           "Bytes/ByteArray/UnderFlow/AllowUnmarshalArrayWithUnderflow",
			unmarshalInput: new([5]byte),
			opts:           []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckUnderflowArrayOnUnmarshaling)},
			dynamoValue: &types.AttributeValueMemberB{
				Value: []byte{'h', 'e'},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindList,
					GoType: reflect.TypeFor[[5]byte](),
					Err:    serializer.ErrUnMarshalArrayUnderflow,
				},
			),
		},
		{
			name:           "Bytes/ByteArray/UnderFlow/CheckOverFlowAndUnderflowOnUnmarshal",
			unmarshalInput: new([5]byte),
			opts:           []serializer.DecoderOption{serializer.WithArrayBoundCheckMode(serializer.CheckOverFlowAndUnderflowArrayOnUnmarshal)},
			dynamoValue: &types.AttributeValueMemberB{
				Value: nil,
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindList,
					GoType: reflect.TypeFor[[5]byte](),
					Err:    serializer.ErrUnMarshalArrayUnderflow,
				},
			),
		},
		{
			name:           "Bytes/Array/InvalidType",
			unmarshalInput: addr(new([3]byte)),
			dynamoValue:    &types.AttributeValueMemberSS{Value: nil},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindStringSet,
					GoType: reflect.TypeFor[[3]byte](),
					Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberSS"),
				},
			),
		},
		{
			name:            "Pointers/NullL0",
			dynamoValue:     &types.AttributeValueMemberNULL{Value: true},
			unmarshalInput:  new(*string),
			expectedGoValue: (*string)(nil),
		},
		{
			name:            "Pointers/NullL1",
			dynamoValue:     &types.AttributeValueMemberNULL{Value: true},
			unmarshalInput:  addr(new(*string)),
			expectedGoValue: (**string)(nil),
		},
		{
			name:            "Pointers/Bool",
			dynamoValue:     &types.AttributeValueMemberBOOL{Value: true},
			unmarshalInput:  addr(new(bool)),
			expectedGoValue: addr(true),
		},
		{
			name:            "Pointers/String",
			dynamoValue:     &types.AttributeValueMemberS{Value: "hello"},
			unmarshalInput:  addr(new(string)),
			expectedGoValue: addr("hello"),
		},
		{
			name:            "Pointers/Bytes",
			dynamoValue:     &types.AttributeValueMemberB{Value: []byte{'h', 'e', 'l', 'l', 'o'}},
			unmarshalInput:  addr(new([]byte)),
			expectedGoValue: addr([]byte("hello")),
		},
		{
			name:            "Pointers/Int",
			dynamoValue:     &types.AttributeValueMemberN{Value: "-123"},
			unmarshalInput:  addr(new(int)),
			expectedGoValue: addr(int(-123)),
		},
		{
			name:            "Pointers/Uint",
			dynamoValue:     &types.AttributeValueMemberN{Value: "123"},
			unmarshalInput:  addr(new(int)),
			expectedGoValue: addr(int(123)),
		},
		{
			name:            "Pointers/Float",
			dynamoValue:     &types.AttributeValueMemberN{Value: "123.456"},
			unmarshalInput:  addr(new(float64)),
			expectedGoValue: addr(float64(123.456)),
		}, {
			name:            "Pointers/Allocate",
			dynamoValue:     &types.AttributeValueMemberS{Value: "hello"},
			unmarshalInput:  addr((*string)(nil)),
			expectedGoValue: addr("hello"),
		},
		{
			name:            "Maps/Empty",
			unmarshalInput:  new(map[string]string),
			expectedGoValue: map[string]string{},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{},
			},
		},
		{
			name:            "Maps/Null",
			unmarshalInput:  new(map[string]string),
			expectedGoValue: map[string]string(nil),
			dynamoValue: &types.AttributeValueMemberNULL{
				Value: true,
			},
		},
		{
			name:            "Maps/String/String",
			unmarshalInput:  new(map[string]int),
			expectedGoValue: map[string]int{"a": 0, "b": 1, "c": 2},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"a": &types.AttributeValueMemberN{Value: "0"},
					"b": &types.AttributeValueMemberN{Value: "1"},
					"c": &types.AttributeValueMemberN{Value: "2"},
				},
			},
		},
		{
			name: "Maps/Bool/String",
			expectedGoValue: map[bool]string{
				true:  "aaa",
				false: "bbb",
			},
			unmarshalInput: new(map[bool]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"true":  &types.AttributeValueMemberS{Value: "aaa"},
					"false": &types.AttributeValueMemberS{Value: "bbb"},
				},
			},
		},
		{
			name:           "Maps/Int/String/InvalidKey",
			unmarshalInput: new(map[int]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"x": &types.AttributeValueMemberS{Value: "aa"},
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[map[int]string](),
					Kind:   serializer.DynamoKindMap,
					Err: serializer.NewUnMarshalError(
						serializer.SerializerErrorData{
							Kind:   serializer.DynamoKindNumber,
							GoType: reflect.TypeFor[int](),
							Err:    fmt.Errorf("strconv.ParseInt: parsing \"x\": invalid syntax"),
						},
					),
				},
			),
		},
		{
			name:           "Maps/UnsupportedValueType/Channel",
			unmarshalInput: new(chan string),
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Err:    errors.ErrUnsupported,
					Kind:   serializer.DynamoKindUnknown,
					GoType: reflect.TypeFor[chan string](),
				},
			),
		},
		{
			name:           "Maps/Uint/String/InvalidKey",
			unmarshalInput: new(map[uint]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"x": &types.AttributeValueMemberS{Value: "aa"},
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[map[uint]string](),
					Kind:   serializer.DynamoKindMap,
					Err: serializer.NewUnMarshalError(
						serializer.SerializerErrorData{
							Kind:   serializer.DynamoKindNumber,
							GoType: reflect.TypeFor[uint](),
							Err:    fmt.Errorf("strconv.ParseUint: parsing \"x\": invalid syntax"),
						},
					),
				},
			),
		},
		{
			name:           "Maps/String/Int/InvalidValue",
			unmarshalInput: new(map[string]int),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"x": &types.AttributeValueMemberB{Value: []byte{'t', 'e', 's', 't'}},
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[map[string]int](),
					Kind:   serializer.DynamoKindMap,
					Err: serializer.NewUnMarshalError(
						serializer.SerializerErrorData{
							Kind:   serializer.DynamoKindBinary,
							GoType: reflect.TypeFor[int](),
							Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberB"),
						},
					),
				},
			),
		},
		{
			name:           "Maps/InvalidKey/Bool",
			unmarshalInput: new(map[bool]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"1222": &types.AttributeValueMemberS{Value: "aaa"},
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[map[bool]string](),
					Err: serializer.NewUnMarshalError(
						serializer.SerializerErrorData{
							Kind:   serializer.DynamoKindBool,
							GoType: reflect.TypeFor[bool](),
							Err:    fmt.Errorf("strconv.ParseBool: parsing \"1222\": invalid syntax"),
						},
					),
				},
			),
		},
		{
			name:           "Maps/InValidKey/Int",
			unmarshalInput: new(map[int8]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"xxx": &types.AttributeValueMemberN{Value: "aaa"},
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[map[int8]string](),
					Err: serializer.NewUnMarshalError(
						serializer.SerializerErrorData{
							Kind:   serializer.DynamoKindNumber,
							GoType: reflect.TypeFor[int8](),
							Err:    fmt.Errorf("strconv.ParseInt: parsing \"xxx\": invalid syntax"),
						},
					),
				},
			),
		},
		{
			name:            "Interfaces/TextUnmarshaler",
			dynamoValue:     &types.AttributeValueMemberNULL{Value: true},
			unmarshalInput:  new(UnmarshalTextToNumber),
			expectedGoValue: UnmarshalTextToNumber{},
		},
		{
			name:           "Maps/TextUnmarshaler/ErrorUnmarshaling",
			unmarshalInput: new(map[UnmarshalTextToNumber]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"xxx": &types.AttributeValueMemberN{Value: "aaa"},
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[map[UnmarshalTextToNumber]string](),
					Err: serializer.NewUnMarshalError(
						serializer.SerializerErrorData{
							GoType: reflect.TypeFor[UnmarshalTextToNumber](),
							Err:    fmt.Errorf("failed_to_parse_number"),
						},
					),
				},
			),
		},
		{
			name:            "Interfaces/BinaryUnMarshaler",
			dynamoValue:     &types.AttributeValueMemberB{Value: []byte("abc")},
			unmarshalInput:  new(binaryUnMarshalerMock),
			expectedGoValue: binaryUnMarshalerMock{Data: []byte(("abc"))},
		},
		{
			name:           "Interfaces/BinaryUnMarshaler/Error",
			dynamoValue:    &types.AttributeValueMemberB{Value: []byte("abc")},
			unmarshalInput: &binaryUnMarshalerMock{ErrInject: true},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindBinary,
					GoType: reflect.TypeFor[binaryUnMarshalerMock](),
					Err:    fmt.Errorf("mock-error"),
				},
			),
		},
		{
			name:           "Maps/UnsupportedKey/channel",
			unmarshalInput: new(map[chan string]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"true":  &types.AttributeValueMemberS{Value: "aaa"},
					"false": &types.AttributeValueMemberS{Value: "bbb"},
				},
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[map[chan string]string](),
					Err:    fmt.Errorf("map key of type chan string is not supported"),
				},
			),
		},
		{
			name:            "Maps/Uint64/String",
			expectedGoValue: map[uint64]string{0: "Zero", math.MaxUint64: "MaxUint64"},
			unmarshalInput:  new(map[uint64]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"0":                    &types.AttributeValueMemberS{Value: "Zero"},
					"18446744073709551615": &types.AttributeValueMemberS{Value: "MaxUint64"},
				},
			},
		},
		{
			name: "Maps/NamedInt64/String",
			expectedGoValue: map[namedInt64]string{
				math.MinInt64: "MinInt64",
				0:             "Zero", math.MaxInt64: "MaxInt64",
			},
			unmarshalInput: new(map[namedInt64]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"-9223372036854775808": &types.AttributeValueMemberS{Value: "MinInt64"},
					"0":                    &types.AttributeValueMemberS{Value: "Zero"},
					"9223372036854775807":  &types.AttributeValueMemberS{Value: "MaxInt64"},
				},
			},
		},
		{
			name:            "Maps/NamedBool/String",
			expectedGoValue: map[namedBool]string{false: "aaa"},
			unmarshalInput:  new(map[namedBool]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"false": &types.AttributeValueMemberS{Value: "aaa"},
				},
			},
		},
		{
			name:            "Maps/Int/String",
			expectedGoValue: map[int64]string{math.MinInt64: "MinInt64", 0: "Zero", math.MaxInt64: "MaxInt64"},
			unmarshalInput:  new(map[int64]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"-9223372036854775808": &types.AttributeValueMemberS{Value: "MinInt64"},
					"0":                    &types.AttributeValueMemberS{Value: "Zero"},
					"9223372036854775807":  &types.AttributeValueMemberS{Value: "MaxInt64"},
				},
			},
		},
		{
			name:           "Maps/InvalidKeyType",
			unmarshalInput: new(map[string]int),
			dynamoValue: &types.AttributeValueMemberBOOL{
				Value: false,
			},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[map[string]int](),
					Kind:   serializer.DynamoKindBool,
					Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberBOOL"),
				},
			),
		},
		{
			name:           "Maps/ValidKey/TextUnmarshalerImpl",
			unmarshalInput: new(map[upperCaseTextUnmarshalerMock]string),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"aAa": &types.AttributeValueMemberS{Value: "test1"},
					"bbB": &types.AttributeValueMemberS{Value: "test2"},
				},
			},
			expectedGoValue: map[upperCaseTextUnmarshalerMock]string{
				upperCaseTextUnmarshalerMock("AAA"): "test1",
				upperCaseTextUnmarshalerMock("BBB"): "test2",
			},
		},
		{
			name: "Structs/Empty",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{},
			},
			unmarshalInput: &structAll{
				String: "hello",
				Map:    map[string]string{},
				Slice:  []string{},
			},
			expectedGoValue: structAll{
				String: "hello",
				Map:    map[string]string{},
				Slice:  []string{},
			},
		},
		{
			name: "Structs/SkipUnknownValues",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"B": &types.AttributeValueMemberN{Value: "10"},
					"A": &types.AttributeValueMemberS{Value: "ABC"},
				},
			},
			unmarshalInput: new(structA),
			expectedGoValue: structA{
				A: "ABC",
			},
		},
		{
			name:            "Structs/Null",
			dynamoValue:     &types.AttributeValueMemberNULL{Value: true},
			unmarshalInput:  &structAll{String: "something"},
			expectedGoValue: structAll{},
		},
		{
			name: "Structs/Normal",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Bool":   &types.AttributeValueMemberBOOL{Value: true},
					"String": &types.AttributeValueMemberS{Value: "hello"},
					"Bytes":  &types.AttributeValueMemberB{Value: []byte{1, 2, 3}},
					"Int":    &types.AttributeValueMemberN{Value: "-64"},
					"Uint":   &types.AttributeValueMemberN{Value: "64"},
					"Float":  &types.AttributeValueMemberN{Value: "3.14159"},
					"Map": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"key": &types.AttributeValueMemberS{
								Value: "value",
							},
						},
					},
					"StructScalars": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"Bool":   &types.AttributeValueMemberBOOL{Value: true},
							"String": &types.AttributeValueMemberS{Value: "hello"},
							"Bytes":  &types.AttributeValueMemberB{Value: []byte{1, 2, 3}},
							"Int":    &types.AttributeValueMemberN{Value: "-64"},
							"Uint":   &types.AttributeValueMemberN{Value: "64"},
							"Float":  &types.AttributeValueMemberN{Value: "3.14159"},
						},
					},
					"StructMaps": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"MapBool":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberBOOL{Value: true}}},
							"MapString": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberS{Value: "hello"}}},
							"MapBytes":  &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberB{Value: []byte{1, 2, 3}}}},
							"MapInt":    &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberN{Value: "-64"}}},
							"MapUint":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberN{Value: "64"}}},
							"MapFloat":  &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberN{Value: "3.14159"}}},
						},
					},
					"StructSlices": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"SliceBool":   &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberBOOL{Value: true}}},
							"SliceString": &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberS{Value: "hello"}}},
							"SliceBytes":  &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberB{Value: []byte{1, 2, 3}}}},
							"SliceInt":    &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberN{Value: "-64"}}},
							"SliceUint":   &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberN{Value: "64"}}},
							"SliceFloat":  &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberN{Value: "3.14159"}}},
						},
					},
					"Slice": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberS{Value: "fizz"},
							&types.AttributeValueMemberS{Value: "buzz"},
						},
					},
					"Array": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberS{Value: "goodbye"},
						},
					},
					"Pointer": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{},
					},
					"Interface": &types.AttributeValueMemberNULL{Value: true},
				},
			},
			unmarshalInput: &structAll{},
			expectedGoValue: structAll{
				Bool:   true,
				String: "hello",
				Bytes:  []byte{1, 2, 3},
				Int:    -64,
				Uint:   +64,
				Float:  3.14159,
				Map:    map[string]string{"key": "value"},
				StructScalars: structScalars{
					Bool:   true,
					String: "hello",
					Bytes:  []byte{1, 2, 3},
					Int:    -64,
					Uint:   +64,
					Float:  3.14159,
				},
				StructMaps: structMaps{
					MapBool:   map[string]bool{"": true},
					MapString: map[string]string{"": "hello"},
					MapBytes:  map[string][]byte{"": {1, 2, 3}},
					MapInt:    map[string]int64{"": -64},
					MapUint:   map[string]uint64{"": +64},
					MapFloat:  map[string]float64{"": 3.14159},
				},
				StructSlices: structSlices{
					SliceBool:   []bool{true},
					SliceString: []string{"hello"},
					SliceBytes:  [][]byte{{1, 2, 3}},
					SliceInt:    []int64{-64},
					SliceUint:   []uint64{+64},
					SliceFloat:  []float64{3.14159},
				},
				Slice:   []string{"fizz", "buzz"},
				Array:   [1]string{"goodbye"},
				Pointer: new(structAll),
			},
		},
		{
			name: "Structs/Merge",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Bool":   &types.AttributeValueMemberBOOL{Value: false},
					"String": &types.AttributeValueMemberS{Value: "goodbye"},
					"Int":    &types.AttributeValueMemberN{Value: "-64"},
					"Float":  &types.AttributeValueMemberN{Value: "3.14159"},
					"Map": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"k2": &types.AttributeValueMemberS{
								Value: "v2",
							},
						},
					},
					"StructScalars": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"Bool":   &types.AttributeValueMemberBOOL{Value: true},
							"String": &types.AttributeValueMemberS{Value: "hello"},
							"Bytes":  &types.AttributeValueMemberB{Value: []byte{1, 2, 3}},
							"Int":    &types.AttributeValueMemberN{Value: "-64"},
						},
					},
					"StructMaps": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"MapBool":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberBOOL{Value: true}}},
							"MapString": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberS{Value: "hello"}}},
							"MapBytes":  &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberB{Value: []byte{1, 2, 3}}}},
							"MapInt":    &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberN{Value: "-64"}}},
							"MapUint":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberN{Value: "64"}}},
							"MapFloat":  &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"": &types.AttributeValueMemberN{Value: "3.14159"}}},
						},
					},
				},
			},
			expectedGoValue: structAll{
				Bool:   false,
				String: "goodbye",
				Bytes:  []byte{1, 2, 3},
				Int:    -64,
				Uint:   +64,
				Float:  3.14159,
				Map:    map[string]string{"k1": "v1", "k2": "v2"},
				StructScalars: structScalars{
					Bool:   true,
					String: "hello",
					Bytes:  []byte{1, 2, 3},
					Int:    -64,
					Uint:   +64,
					Float:  3.14159,
				},
				StructMaps: structMaps{
					MapBool:   map[string]bool{"": true},
					MapString: map[string]string{"": "hello"},
					MapBytes:  map[string][]byte{"": {1, 2, 3}},
					MapInt:    map[string]int64{"": -64},
					MapUint:   map[string]uint64{"": +64},
					MapFloat:  map[string]float64{"": 3.14159},
				},
				StructSlices: structSlices{
					SliceBool:   []bool{true},
					SliceString: []string{"hello"},
					SliceBytes:  [][]byte{{1, 2, 3}},
					SliceInt:    []int64{-64},
					SliceUint:   []uint64{+64},
					SliceFloat:  []float64{3.14159},
				},
				Slice:     []string{"fizz", "buzz"},
				Array:     [1]string{"goodbye"},
				Pointer:   new(structAll),
				Interface: map[string]string{"k1": "v1", "k2": "v2"},
			},
			unmarshalInput: &structAll{
				Bool:   false,
				String: "goodbye",
				Bytes:  []byte{1, 2, 3},
				Int:    -64,
				Uint:   +64,
				Float:  3.14159,
				Map:    map[string]string{"k1": "v1", "k2": "v2"},
				StructScalars: structScalars{
					Bool:   true,
					String: "hello",
					Bytes:  []byte{1, 2, 3},
					Int:    -64,
					Uint:   +64,
					Float:  3.14159,
				},
				StructMaps: structMaps{
					MapBool:   map[string]bool{"": true},
					MapString: map[string]string{"": "hello"},
					MapBytes:  map[string][]byte{"": {1, 2, 3}},
					MapInt:    map[string]int64{"": -64},
					MapUint:   map[string]uint64{"": +64},
					MapFloat:  map[string]float64{"": 3.14159},
				},
				StructSlices: structSlices{
					SliceBool:   []bool{true},
					SliceString: []string{"hello"},
					SliceBytes:  [][]byte{{1, 2, 3}},
					SliceInt:    []int64{-64},
					SliceUint:   []uint64{+64},
					SliceFloat:  []float64{3.14159},
				},
				Slice:     []string{"fizz", "buzz"},
				Array:     [1]string{"goodbye"},
				Pointer:   new(structAll),
				Interface: map[string]string{"k1": "v1", "k2": "v2"},
			},
		},
		{
			name: "Structs/Inline/Alloc",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"E": &types.AttributeValueMemberS{Value: ""},
					"F": &types.AttributeValueMemberS{Value: ""},
					"G": &types.AttributeValueMemberS{Value: ""},
					"A": &types.AttributeValueMemberS{Value: ""},
					"B": &types.AttributeValueMemberS{Value: ""},
					"D": &types.AttributeValueMemberS{Value: ""},
				},
			},
			unmarshalInput: &structInlined{},
			expectedGoValue: structInlined{
				X: structInlinedL1{
					X:            &structInlinedL2{},
					StructEmbed1: StructEmbed1{},
				},
				StructEmbed2: &StructEmbed2{},
			},
		},
		{
			name: "Structs/Inline/Zero",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"D": &types.AttributeValueMemberS{Value: ""},
				}},
			unmarshalInput:  &structInlined{},
			expectedGoValue: structInlined{},
		},
		{
			name: "Structs/Inline/NonZero",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"E": &types.AttributeValueMemberS{Value: "E3"},
					"F": &types.AttributeValueMemberS{Value: "F3"},
					"G": &types.AttributeValueMemberS{Value: "G3"},
					"A": &types.AttributeValueMemberS{Value: "A1"},
					"B": &types.AttributeValueMemberS{Value: "B1"},
					"D": &types.AttributeValueMemberS{Value: "D2"},
				},
			},
			unmarshalInput: &structInlined{},
			expectedGoValue: structInlined{
				X: structInlinedL1{
					X: &structInlinedL2{
						A: "A1",
						B: "B1",
					},
					StructEmbed1: StructEmbed1{
						D: "D2",
					},
				},
				StructEmbed2: &StructEmbed2{
					E: "E3",
					F: "F3",
					G: "G3",
				},
			},
		},
		{
			name: "Structs/Inline/Merge",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"E": &types.AttributeValueMemberS{Value: "E3"},
					"F": &types.AttributeValueMemberS{Value: "F3"},
					"G": &types.AttributeValueMemberS{Value: "G3"},
					"A": &types.AttributeValueMemberS{Value: "A1"},
					"B": &types.AttributeValueMemberS{Value: "B1"},
					"D": &types.AttributeValueMemberS{Value: "D2"},
				},
			},
			unmarshalInput: &structInlined{
				X: structInlinedL1{
					X:            &structInlinedL2{B: "##", C: "C1"},
					StructEmbed1: StructEmbed1{C: "C2", E: "E2"},
				},
				StructEmbed2: &StructEmbed2{E: "##", G: "G3"},
			},
			expectedGoValue: structInlined{
				X: structInlinedL1{
					X:            &structInlinedL2{A: "A1", B: "B1", C: "C1"},
					StructEmbed1: StructEmbed1{C: "C2", D: "D2", E: "E2"},
				},
				StructEmbed2: &StructEmbed2{E: "E3", F: "F3", G: "G3"},
			},
		},
		{
			name:           "Structs/Invalid/Conflicting",
			unmarshalInput: new(structConflicting),
			dynamoValue:    &types.AttributeValueMemberM{Value: nil},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[structConflicting](),
					Kind:   serializer.DynamoKindMap,
					Err:    fmt.Errorf("Go struct fields A and B conflict over dynamodbav object name \"conflict\""),
				},
			),
		},
		{
			name:            "Structs/CustomUnmarshaler",
			expectedGoValue: struct{ Test int }{Test: 2},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Test": &types.AttributeValueMemberBOOL{
						Value: true,
					},
				},
			},
			opts: []serializer.DecoderOption{
				serializer.WithUnmarshalers(
					serializer.UnMarshalFuncV1(
						func(av types.AttributeValue, t *struct{ Test int }) error {
							casted, ok := av.(*types.AttributeValueMemberM)
							if !ok {
								return fmt.Errorf("expected *types.AttributeValueMemberM got %T", av)
							}
							if len(casted.Value) != 1 {
								return fmt.Errorf("expected map to contain single item but map value is %#v", casted.Value)
							}
							tmp, ok := casted.Value["Test"]
							if !ok {
								return fmt.Errorf("Test key doesnt exists")
							}
							castedBool, ok := tmp.(*types.AttributeValueMemberBOOL)
							if !ok {
								return fmt.Errorf("Test key is not bool")
							}
							if castedBool.Value {
								t.Test = 2
							} else {
								t.Test = 6
							}
							return nil
						},
					),
				),
			},
			unmarshalInput: &struct{ Test int }{},
		},
		{
			name:            "Interfaces/Empty/Null",
			dynamoValue:     &types.AttributeValueMemberNULL{Value: true},
			unmarshalInput:  new(any),
			expectedGoValue: nil,
		},
		{
			name:            "Interfaces/NonEmpty/Null",
			dynamoValue:     &types.AttributeValueMemberNULL{Value: true},
			unmarshalInput:  new(io.Reader),
			expectedGoValue: nil,
		},
		{
			name:           "Interfaces/Empty/False",
			dynamoValue:    &types.AttributeValueMemberBOOL{Value: false},
			unmarshalInput: new(any),
			expectedGoValue: func() any {
				var vi any = false
				return vi
			}(),
		},

		{
			name:            "Interfaces/UnmarshalTextToNumber/InvalidString",
			dynamoValue:     &types.AttributeValueMemberS{Value: "abcsa"},
			unmarshalInput:  new(UnmarshalTextToNumber),
			expectedGoValue: UnmarshalTextToNumber{},
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindString,
					GoType: reflect.TypeFor[UnmarshalTextToNumber](),
					Err:    fmt.Errorf("failed_to_parse_number"),
				},
			),
		},
		{
			name:            "Interfaces/UnmarshalTextToNumber",
			dynamoValue:     &types.AttributeValueMemberS{Value: "10"},
			unmarshalInput:  new(UnmarshalTextToNumber),
			expectedGoValue: UnmarshalTextToNumber{secret: 10},
		},
		{
			name:           "Interfaces/UnsupportedType/UnmarshalTextToNumber",
			dynamoValue:    &types.AttributeValueMemberBS{Value: [][]byte{}},
			unmarshalInput: new(UnmarshalTextToNumber),
			expectedError: serializer.NewUnMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindBinarySet,
					GoType: reflect.TypeFor[UnmarshalTextToNumber](),
					Err:    fmt.Errorf("unsupported AttributeValue type, *types.AttributeValueMemberBS"),
				},
			),
		},
		{
			name:            "Interfaces/Empty/False",
			dynamoValue:     &types.AttributeValueMemberBOOL{Value: false},
			unmarshalInput:  new(any),
			expectedGoValue: any(false),
		},
		{
			name:            "Interfaces/Empty/True",
			dynamoValue:     &types.AttributeValueMemberBOOL{Value: true},
			unmarshalInput:  new(any),
			expectedGoValue: any(true),
		},
		{
			name:            "Interfaces/Empty/String",
			dynamoValue:     &types.AttributeValueMemberS{Value: "string"},
			unmarshalInput:  new(any),
			expectedGoValue: any("string"),
		},
		{
			name:            "Interfaces/Empty/Number",
			dynamoValue:     &types.AttributeValueMemberN{Value: "3.14159"},
			unmarshalInput:  new(any),
			expectedGoValue: any(3.14159),
		},
		{
			name:            "Interfaces/NamedAny/String",
			dynamoValue:     &types.AttributeValueMemberS{Value: "string"},
			unmarshalInput:  new(namedAny),
			expectedGoValue: namedAny("string"),
		},
		{
			name: "Interfaces/Merge/Map",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"k2": &types.AttributeValueMemberS{Value: "v2"},
				}},
			unmarshalInput:  &map[string]any{"k1": "v1"},
			expectedGoValue: map[string]any{"k1": "v1", "k2": "v2"},
		},
		{
			name: "Interfaces/Merge/Struct",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Array": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberS{Value: "goodbye"},
						}},
				}},
			unmarshalInput:  &structAll{String: "hello"},
			expectedGoValue: structAll{String: "hello", Array: [1]string{"goodbye"}},
		},
		{
			name:            "Interfaces/Merge/NamedInt",
			dynamoValue:     &types.AttributeValueMemberN{Value: "52"},
			unmarshalInput:  addr(any(20)),
			expectedGoValue: 52,
		},
		{
			name: ("Interfaces/Empty/Object"),
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"k": &types.AttributeValueMemberS{Value: "v"},
				},
			},
			unmarshalInput:  new(any),
			expectedGoValue: any(map[string]any{"k": "v"}),
		},
		{
			name: "Interfaces/Empty/Array",
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "k"},
				},
			},
			unmarshalInput:  new(any),
			expectedGoValue: any([]any{"k"}),
		},
		{
			name: "Interfaces/Any",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberNULL{Value: true},
							&types.AttributeValueMemberBOOL{Value: false},
							&types.AttributeValueMemberBOOL{Value: true},
							&types.AttributeValueMemberS{Value: ""},
							&types.AttributeValueMemberN{Value: "0"},
							&types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{},
							},
							&types.AttributeValueMemberL{Value: []types.AttributeValue{}},
						},
					},
				},
			},

			unmarshalInput: new(struct{ X any }),
			expectedGoValue: struct{ X any }{
				[]any{
					nil,
					false,
					true,
					"",
					0.0,
					map[string]any{},
					[]any{},
				},
			},
		},
		{
			name: "Interfaces/Any/Named",
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberNULL{Value: true},
							&types.AttributeValueMemberBOOL{Value: false},
							&types.AttributeValueMemberBOOL{Value: true},
							&types.AttributeValueMemberS{Value: ""},
							&types.AttributeValueMemberN{Value: "0"},
							&types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{},
							},
							&types.AttributeValueMemberL{Value: []types.AttributeValue{}},
						},
					},
				},
			},
			unmarshalInput: new(struct{ X namedAny }),
			expectedGoValue: struct{ X namedAny }{
				[]any{
					nil,
					false,
					true,
					"",
					0.0,
					map[string]any{},
					[]any{},
				},
			},
		},
	}

	for _, testcase := range cases {
		t.Run(testcase.name,
			func(t *testing.T) {
				r := require.New(t)
				decoder := serializer.NewDecoder(testcase.opts...)
				err := decoder.Unmarshal(testcase.dynamoValue, testcase.unmarshalInput)
				if testcase.expectedError != nil {
					r.Error(err)
					r.EqualError(err, testcase.expectedError.Error())
				} else {
					r.NoError(err)
					inputValue := reflect.ValueOf(testcase.unmarshalInput)
					got := inputValue.Elem().Interface()
					r.Equal(testcase.expectedGoValue, got)
				}
			},
		)
	}
}

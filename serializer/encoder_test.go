package serializer_test

import (
	"encoding"
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/serializer"
	"github.com/stretchr/testify/require"
)

func addr[T any](t T) *T {
	return &t
}

type namedAny any
type namedBool bool
type namedString string
type namedInt int
type namedInt64 int64
type namedFloat64 float64
type namedUint64 uint64
type namedByte byte
type namedEmptyStruct struct{}

type lowerCasedString string

var _ encoding.TextMarshaler = (lowerCasedString)("")

func (k lowerCasedString) MarshalText() (text []byte, err error) {
	text = []byte(strings.ToLower(string(k)))
	return text, nil
}

type structAll struct {
	Bool          bool
	String        string
	Bytes         []byte
	Int           int64
	Uint          uint64
	Float         float64
	Map           map[string]string
	StructScalars structScalars
	StructMaps    structMaps
	StructSlices  structSlices
	Slice         []string
	Array         [1]string
	Pointer       *structAll
	Interface     any
}

type structScalars struct {
	unexported bool
	Ignored    bool `dynamodbav:"-"`

	Bool   bool
	String string
	Bytes  []byte
	Int    int64
	Uint   uint64
	Float  float64
}
type structSlices struct {
	unexported bool
	Ignored    bool `dynamodbav:"-"`

	SliceBool   []bool
	SliceString []string
	SliceBytes  [][]byte
	SliceInt    []int64
	SliceUint   []uint64
	SliceFloat  []float64
}
type structMaps struct {
	unexported bool
	Ignored    bool `dynamodbav:"-"`

	MapBool   map[string]bool
	MapString map[string]string
	MapBytes  map[string][]byte
	MapInt    map[string]int64
	MapUint   map[string]uint64
	MapFloat  map[string]float64
}

type structOmitZeroAll struct {
	Bool          bool               `dynamodbav:",omitzero"`
	String        string             `dynamodbav:",omitzero"`
	Bytes         []byte             `dynamodbav:",omitzero"`
	Int           int64              `dynamodbav:",omitzero"`
	Uint          uint64             `dynamodbav:",omitzero"`
	Float         float64            `dynamodbav:",omitzero"`
	Map           map[string]string  `dynamodbav:",omitzero"`
	StructScalars structScalars      `dynamodbav:",omitzero"`
	StructMaps    structMaps         `dynamodbav:",omitzero"`
	StructSlices  structSlices       `dynamodbav:",omitzero"`
	Slice         []string           `dynamodbav:",omitzero"`
	Array         [1]string          `dynamodbav:",omitzero"`
	Pointer       *structOmitZeroAll `dynamodbav:",omitzero"`
	Interface     any                `dynamodbav:",omitzero"`
}

type structUnexportedIgnored struct {
	ignored string `dynamodbav:"-"`
}

type unexportedEmbeddedStruct struct {
	X string
	y string
}

type structOmitZeroMethodAll struct {
	ValueAlwaysZero                 valueAlwaysZero     `dynamodbav:",omitzero"`
	ValueNeverZero                  valueNeverZero      `dynamodbav:",omitzero"`
	PointerAlwaysZero               pointerAlwaysZero   `dynamodbav:",omitzero"`
	PointerNeverZero                pointerNeverZero    `dynamodbav:",omitzero"`
	PointerValueAlwaysZero          *valueAlwaysZero    `dynamodbav:",omitzero"`
	PointerValueNeverZero           *valueNeverZero     `dynamodbav:",omitzero"`
	PointerPointerAlwaysZero        *pointerAlwaysZero  `dynamodbav:",omitzero"`
	PointerPointerNeverZero         *pointerNeverZero   `dynamodbav:",omitzero"`
	PointerPointerValueAlwaysZero   **valueAlwaysZero   `dynamodbav:",omitzero"`
	PointerPointerValueNeverZero    **valueNeverZero    `dynamodbav:",omitzero"`
	PointerPointerPointerAlwaysZero **pointerAlwaysZero `dynamodbav:",omitzero"`
	PointerPointerPointerNeverZero  **pointerNeverZero  `dynamodbav:",omitzero"`
}

type valueAlwaysZero string

func (valueAlwaysZero) IsZero() bool {
	return true
}

type valueNeverZero string

func (valueNeverZero) IsZero() bool {
	return false
}

type pointerAlwaysZero string

func (*pointerAlwaysZero) IsZero() bool {
	return true
}

type pointerNeverZero string

func (*pointerNeverZero) IsZero() bool {
	return false
}

type structOmitZeroMethodInterfaceAll struct {
	ValueAlwaysZero          serializer.IsZeroer `dynamodbav:",omitzero"`
	ValueNeverZero           serializer.IsZeroer `dynamodbav:",omitzero"`
	PointerValueAlwaysZero   serializer.IsZeroer `dynamodbav:",omitzero"`
	PointerValueNeverZero    serializer.IsZeroer `dynamodbav:",omitzero"`
	PointerPointerAlwaysZero serializer.IsZeroer `dynamodbav:",omitzero"`
	PointerPointerNeverZero  serializer.IsZeroer `dynamodbav:",omitzero"`
}

type structOmitZeroEmptyAll struct {
	Bool      bool                    `dynamodbav:",omitzero,omitempty"`
	String    string                  `dynamodbav:",omitzero,omitempty"`
	Bytes     []byte                  `dynamodbav:",omitzero,omitempty"`
	Int       int64                   `dynamodbav:",omitzero,omitempty"`
	Uint      uint64                  `dynamodbav:",omitzero,omitempty"`
	Float     float64                 `dynamodbav:",omitzero,omitempty"`
	Map       map[string]string       `dynamodbav:",omitzero,omitempty"`
	Slice     []string                `dynamodbav:",omitzero,omitempty"`
	Array     [1]string               `dynamodbav:",omitzero,omitempty"`
	Pointer   *structOmitZeroEmptyAll `dynamodbav:",omitzero,omitempty"`
	Interface any                     `dynamodbav:",omitzero,omitempty"`
}

type structOmitEmptyAll struct {
	Bool                  bool                    `dynamodbav:",omitempty"`
	PointerBool           *bool                   `dynamodbav:",omitempty"`
	String                string                  `dynamodbav:",omitempty"`
	StringEmpty           stringMarshalEmpty      `dynamodbav:",omitempty"`
	StringNonEmpty        stringMarshalNonEmpty   `dynamodbav:",omitempty"`
	PointerString         *string                 `dynamodbav:",omitempty"`
	PointerStringEmpty    *stringMarshalEmpty     `dynamodbav:",omitempty"`
	PointerStringNonEmpty *stringMarshalNonEmpty  `dynamodbav:",omitempty"`
	Bytes                 []byte                  `dynamodbav:",omitempty"`
	BytesEmpty            bytesMarshalEmpty       `dynamodbav:",omitempty"`
	BytesNonEmpty         bytesMarshalNonEmpty    `dynamodbav:",omitempty"`
	PointerBytes          *[]byte                 `dynamodbav:",omitempty"`
	PointerBytesEmpty     *bytesMarshalEmpty      `dynamodbav:",omitempty"`
	PointerBytesNonEmpty  *bytesMarshalNonEmpty   `dynamodbav:",omitempty"`
	Float                 float64                 `dynamodbav:",omitempty"`
	PointerFloat          *float64                `dynamodbav:",omitempty"`
	Map                   map[string]string       `dynamodbav:",omitempty"`
	MapEmpty              mapMarshalEmpty         `dynamodbav:",omitempty"`
	MapNonEmpty           mapMarshalNonEmpty      `dynamodbav:",omitempty"`
	PointerMap            *map[string]string      `dynamodbav:",omitempty"`
	PointerMapEmpty       *mapMarshalEmpty        `dynamodbav:",omitempty"`
	PointerMapNonEmpty    *mapMarshalNonEmpty     `dynamodbav:",omitempty"`
	Slice                 []string                `dynamodbav:",omitempty"`
	SliceEmpty            sliceMarshalEmpty       `dynamodbav:",omitempty"`
	SliceNonEmpty         sliceMarshalNonEmpty    `dynamodbav:",omitempty"`
	PointerSlice          *[]string               `dynamodbav:",omitempty"`
	PointerSliceEmpty     *sliceMarshalEmpty      `dynamodbav:",omitempty"`
	PointerSliceNonEmpty  *sliceMarshalNonEmpty   `dynamodbav:",omitempty"`
	Pointer               *structOmitZeroEmptyAll `dynamodbav:",omitempty"`
	Interface             any                     `dynamodbav:",omitempty"`
}

type stringMarshalEmpty string

func (stringMarshalEmpty) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return &types.AttributeValueMemberS{Value: ""}, nil
}

type stringMarshalNonEmpty string

func (stringMarshalNonEmpty) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return &types.AttributeValueMemberS{Value: "value"}, nil
}

type bytesMarshalEmpty []byte

func (bytesMarshalEmpty) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return &types.AttributeValueMemberB{Value: []byte{}}, nil
}

type bytesMarshalNonEmpty []byte

func (bytesMarshalNonEmpty) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return &types.AttributeValueMemberB{Value: []byte("value")}, nil
}

type mapMarshalEmpty map[string]string

func (mapMarshalEmpty) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return &types.AttributeValueMemberM{}, nil
}

type mapMarshalNonEmpty map[string]string

func (mapMarshalNonEmpty) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"key": &types.AttributeValueMemberS{Value: "value"}}}, nil
}

type sliceMarshalEmpty []string

func (sliceMarshalEmpty) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return &types.AttributeValueMemberL{Value: make([]types.AttributeValue, 0)}, nil
}

type sliceMarshalNonEmpty []string

func (sliceMarshalNonEmpty) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return &types.AttributeValueMemberL{Value: []types.AttributeValue{
		&types.AttributeValueMemberS{Value: "value"},
	}}, nil
}

type cyclicA struct {
	B1 cyclicB `dynamodbav:",inline"`
	B2 cyclicB `dynamodbav:",inline"`
}

type cyclicB struct {
	F int
	A *cyclicA `dynamodbav:",inline"`
}
type structConflicting struct {
	A string `dynamodbav:"conflict"`
	B string `dynamodbav:"conflict"`
}

type structIgnoredUnexportedEmbedded struct {
	unexportedEmbeddedStruct `dynamodbav:"-"`
}

type structUnexportedTag struct {
	unexported string `dynamodbav:"new_name"`
}

type structUnexportedEmbedded struct {
	namedString
}

type structIgnoreUnexportedUntagged struct {
	x int
	Y int
}

type DuplicateSetTags struct {
	Set []string `dynamodbav:",set,set"`
}

// field e will not apperar in output - 2 different sources from same depth
type structInlined struct {
	X             structInlinedL1 `dynamodbav:",inline"`
	*StructEmbed2                 // implicit inline
}

type structInlinedL1 struct {
	X            *structInlinedL2 `dynamodbav:",inline"`
	StructEmbed1 `dynamodbav:",inline"`
}

type structInlinedL2 struct {
	A string
	B string
	C string
}

type StructEmbed1 struct {
	C string
	D string
	E string
}
type StructEmbed2 struct {
	E string
	F string
	G string
}

type StructEmbed1Named struct {
	C string `dynamodbav:"C,"`
	D string `dynamodbav:"D,"`
	E string `dynamodbav:"E,"`
}
type StructEmbed2Named struct {
	E string `dynamodbav:"E,"`
	F string `dynamodbav:"F,"`
	G string `dynamodbav:"G,"`
}

type Embedded1Val struct {
	X int
}
type s1ValueEmbedded struct {
	Embedded1Val
	Y string
}
type s2ValueEmbedded struct {
	Embedded1Val
	X bool
	Y string
}
type sPtrValueEmbedded struct {
	*Embedded1Val
	Z bool
}

var _ attributevalue.Marshaler = MockDynamoMarshaler{}

type MockDynamoMarshaler struct {
	Value types.AttributeValue
	Err   error
}

func (m MockDynamoMarshaler) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return m.Value, m.Err
}

var _ attributevalue.Marshaler = (*MockDynamoMarshaler)(nil)

type MockDynamoMarshalerPtr struct {
	Value types.AttributeValue
	Err   error
}

func (m *MockDynamoMarshalerPtr) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return m.Value, m.Err
}

type embeddedMockMarshaler struct {
	MockAvMarshaler MockDynamoMarshaler `dynamodbav:",inline"`
}

type StructNoExportedFields struct {
	x int
}

var _ encoding.BinaryMarshaler = binaryMarshalerMock{}

type binaryMarshalerMock struct {
	Value string
	Err   error
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (b binaryMarshalerMock) MarshalBinary() (data []byte, err error) {
	return []byte(b.Value), b.Err
}

var _ encoding.BinaryMarshaler = (*binaryMarshalerMockPtr)(nil)

type binaryMarshalerMockPtr struct {
	Value string
	Err   error
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (b *binaryMarshalerMockPtr) MarshalBinary() (data []byte, err error) {
	return []byte(b.Value), b.Err
}

var _ encoding.TextMarshaler = TextMarshalerMock{}

type TextMarshalerMock struct {
	Value string
	Err   error
}

func (b TextMarshalerMock) MarshalText() (data []byte, err error) {
	return []byte(b.Value), b.Err
}

var _ encoding.TextMarshaler = (*TextMarshalerMockPtr)(nil)

type TextMarshalerMockPtr struct {
	Value string
	Err   error
}

// MarshalText implements encoding.TextMarshaler.
func (t *TextMarshalerMockPtr) MarshalText() (text []byte, err error) {
	return []byte(t.Value), t.Err
}

var _ encoding.TextMarshaler = TextMarshalerMockBytes{}

type TextMarshalerMockBytes struct {
	Value []byte
	Err   error
}

func (b TextMarshalerMockBytes) MarshalText() (data []byte, err error) {
	return b.Value, b.Err
}

func TestSerializer(t *testing.T) {
	type encoderTestCase struct {
		name          string
		opts          []serializer.EncoderOption
		goValue       any
		dynamoValue   types.AttributeValue
		expectedError error
	}

	cases := []encoderTestCase{
		{
			name:        "Nil",
			goValue:     nil,
			dynamoValue: &types.AttributeValueMemberNULL{Value: true},
		},
		{
			name:        "Bool",
			goValue:     true,
			dynamoValue: &types.AttributeValueMemberBOOL{Value: true},
		},
		{
			name:    "Bools",
			goValue: []bool{true, false},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberBOOL{Value: true},
					&types.AttributeValueMemberBOOL{Value: false},
				},
			},
		},
		{
			name:    "Bools/Named",
			goValue: []namedBool{namedBool(true), false},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberBOOL{Value: true},
					&types.AttributeValueMemberBOOL{Value: false},
				},
			},
		},
		{
			name:    "Slices/Empty",
			goValue: []bool{},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{},
			},
		},
		{
			name:    "Slices/UnsupportedType/Channel",
			goValue: [](chan string){nil},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[[]chan string](),
					Kind:   serializer.DynamoKindList,
					Err: serializer.NewMarshalError(
						serializer.SerializerErrorData{
							GoType: reflect.TypeFor[chan string](),
							Err:    errors.ErrUnsupported,
						},
					),
				},
			),
		},
		{
			name: "Slices/Set/EmptyEncodedAsNull",
			goValue: struct {
				StringSet []string  `dynamodbav:",set"`
				IntSet    []int     `dynamodbav:",set"`
				UIntSet   []uint64  `dynamodbav:",set"`
				FloatSet  []float64 `dynamodbav:",set"`
				BinarySet [][]byte  `dynamodbav:",set"`
			}{
				StringSet: []string{},
				IntSet:    []int{},
				UIntSet:   []uint64{},
				FloatSet:  []float64{},
				BinarySet: [][]byte{},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"StringSet": &types.AttributeValueMemberNULL{Value: true},
					"IntSet":    &types.AttributeValueMemberNULL{Value: true},
					"UIntSet":   &types.AttributeValueMemberNULL{Value: true},
					"FloatSet":  &types.AttributeValueMemberNULL{Value: true},
					"BinarySet": &types.AttributeValueMemberNULL{Value: true},
				},
			},
		},
		{
			name: "Slices/Set/Uint16/OmitEmpty",
			goValue: struct {
				Uint16Set         []uint16 `dynamodbav:",set,omitempty"`
				Uint16SetNotEmpty []uint16 `dynamodbav:",set,omitempty"`
				Uint16SetNil      []uint16 `dynamodbav:",set,omitempty"`
			}{
				Uint16Set:         make([]uint16, 0),
				Uint16SetNotEmpty: []uint16{1, 5, 20},
				Uint16SetNil:      nil,
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Uint16SetNotEmpty": &types.AttributeValueMemberNS{
						Value: []string{"1", "5", "20"},
					},
				},
			},
		},
		{
			name: "Slices/Set/Int64/OmitEmpty",
			goValue: struct {
				Uint16Set         []int64 `dynamodbav:",set,omitempty"`
				Uint16SetNotEmpty []int64 `dynamodbav:",set,omitempty"`
				Uint16SetNil      []int64 `dynamodbav:",set,omitempty"`
			}{
				Uint16Set:         make([]int64, 0),
				Uint16SetNotEmpty: []int64{1, 5, 20},
				Uint16SetNil:      nil,
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Uint16SetNotEmpty": &types.AttributeValueMemberNS{
						Value: []string{"1", "5", "20"},
					},
				},
			},
		},
		{
			name: "Slices/Set/Float32/OmitEmpty",
			goValue: struct {
				Uint16Set         []float32 `dynamodbav:",set,omitempty"`
				Uint16SetNotEmpty []float32 `dynamodbav:",set,omitempty"`
				Uint16SetNil      []float32 `dynamodbav:",set,omitempty"`
			}{
				Uint16Set:         make([]float32, 0),
				Uint16SetNotEmpty: []float32{1, 5, 20},
				Uint16SetNil:      nil,
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Uint16SetNotEmpty": &types.AttributeValueMemberNS{
						Value: []string{"1", "5", "20"},
					},
				},
			},
		},
		{
			name: "Slices/Set/Binary/OmitEmpty",
			goValue: struct {
				BinarySet         [][]byte `dynamodbav:",set,omitempty"`
				BinarySetNotEmpty [][]byte `dynamodbav:",set,omitempty"`
				BinarySetNil      [][]byte `dynamodbav:",set,omitempty"`
			}{
				BinarySet:         make([][]byte, 0),
				BinarySetNotEmpty: [][]byte{[]byte("test1"), []byte("test2")},
				BinarySetNil:      nil,
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"BinarySetNotEmpty": &types.AttributeValueMemberBS{
						Value: [][]byte{
							[]byte("test1"), []byte("test2"),
						},
					},
				},
			},
		},
		{
			name: "Slices/Set/String/OmitEmpty",
			goValue: struct {
				StringSet         []string `dynamodbav:",set,omitempty"`
				StringSetNotEmpty []string `dynamodbav:",set,omitempty"`
				StringSetNil      []string `dynamodbav:",set,omitempty"`
			}{
				StringSet:         make([]string, 0),
				StringSetNotEmpty: []string{"a", "b"},
				StringSetNil:      nil,
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"StringSetNotEmpty": &types.AttributeValueMemberSS{Value: []string{"a", "b"}},
				},
			},
		},
		{
			name: "Slices/Set/ZeroEncodedAsNull",
			goValue: struct {
				StringSet []string `dynamodbav:",set"`
				IntSet    []int    `dynamodbav:",set"`
				BinarySet [][]byte `dynamodbav:",set"`
			}{},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"StringSet": &types.AttributeValueMemberNULL{Value: true},
					"IntSet":    &types.AttributeValueMemberNULL{Value: true},
					"BinarySet": &types.AttributeValueMemberNULL{Value: true},
				},
			},
		},
		{
			name:        "Arrays/Empty",
			goValue:     [0]struct{}{},
			dynamoValue: &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
		},
		{
			name: "Arrays/SkipEmptyElement",
			goValue: [1]MockDynamoMarshaler{
				{Value: nil}, //will be omitted!
			},
			dynamoValue: &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
		},
		{
			name:    "Arrays/Bool",
			goValue: [2]bool{false, true},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberBOOL{Value: false},
					&types.AttributeValueMemberBOOL{Value: true},
				},
			},
		},
		{
			name:    "Arrays/String",
			goValue: [2]string{"test1", "test2"},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "test1"},
					&types.AttributeValueMemberS{Value: "test2"},
				},
			},
		},
		{
			name:    "Arrays/Bytes",
			goValue: [2][]byte{[]byte("test1"), []byte("test2")},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberB{Value: []byte("test1")},
					&types.AttributeValueMemberB{Value: []byte("test2")},
				},
			},
		},
		{
			name:    "Arrays/Int",
			goValue: [2]int64{math.MinInt64, math.MaxInt64},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberN{Value: "-9223372036854775808"},
					&types.AttributeValueMemberN{Value: "9223372036854775807"},
				},
			},
		},
		{
			name:    "Arrays/Uint",
			goValue: [2]uint64{0, math.MaxUint64},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberN{Value: "0"},
					&types.AttributeValueMemberN{Value: "18446744073709551615"},
				},
			},
		},
		{
			name:    "Arrays/Float",
			goValue: [2]float64{-math.MaxFloat64, +math.MaxFloat64},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberN{Value: "-1.7976931348623157e+308"},
					&types.AttributeValueMemberN{Value: "1.7976931348623157e+308"},
				},
			},
		},
		{
			name:    "Arrays/UnsupportedType/Channel",
			goValue: [1]chan<- string{},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[[1]chan<- string](),
					Kind:   serializer.DynamoKindList,
					Err: serializer.NewMarshalError(
						serializer.SerializerErrorData{
							Kind:   serializer.DynamoKindUnknown,
							GoType: reflect.TypeFor[chan<- string](),
							Err:    errors.ErrUnsupported,
						},
					),
				},
			),
		},
		{
			name:    "Strings",
			goValue: []string{"", "abc", "hello", "世界"},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: ""},
					&types.AttributeValueMemberS{Value: "abc"},
					&types.AttributeValueMemberS{Value: "hello"},
					&types.AttributeValueMemberS{Value: "世界"},
				},
			},
		},
		{
			name:    "Strings/Named",
			goValue: []namedString{"", "abc", "hello", "世界"},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: ""},
					&types.AttributeValueMemberS{Value: "abc"},
					&types.AttributeValueMemberS{Value: "hello"},
					&types.AttributeValueMemberS{Value: "世界"},
				},
			},
		},
		{
			name:        "Ints/Int",
			goValue:     126,
			dynamoValue: &types.AttributeValueMemberN{Value: "126"},
		},
		{
			name:        "Ints/NamedInt",
			goValue:     namedInt(126),
			dynamoValue: &types.AttributeValueMemberN{Value: "126"},
		},
		{
			name:        "Ints/Int8",
			goValue:     int8(10),
			dynamoValue: &types.AttributeValueMemberN{Value: "10"},
		},
		{
			name:        "Ints/Int8/Min",
			goValue:     math.MinInt8,
			dynamoValue: &types.AttributeValueMemberN{Value: "-128"},
		},
		{
			name:        "Ints/Int8/Max",
			goValue:     math.MaxInt8,
			dynamoValue: &types.AttributeValueMemberN{Value: "127"},
		},
		{
			name:        "Ints/Int16",
			goValue:     int16(10),
			dynamoValue: &types.AttributeValueMemberN{Value: "10"},
		},
		{
			name:        "Ints/Int16/Min",
			goValue:     math.MinInt16,
			dynamoValue: &types.AttributeValueMemberN{Value: "-32768"},
		},
		{
			name:        "Ints/Int16/Max",
			goValue:     math.MaxInt16,
			dynamoValue: &types.AttributeValueMemberN{Value: "32767"},
		},
		{
			name:        "Ints/Int32",
			goValue:     int32(10),
			dynamoValue: &types.AttributeValueMemberN{Value: "10"},
		},
		{
			name:        "Ints/Int32/Min",
			goValue:     math.MinInt32,
			dynamoValue: &types.AttributeValueMemberN{Value: "-2147483648"},
		},
		{
			name:        "Ints/Int32/Max",
			goValue:     math.MaxInt32,
			dynamoValue: &types.AttributeValueMemberN{Value: "2147483647"},
		},
		{
			name:        "Ints/Int64",
			goValue:     int64(10),
			dynamoValue: &types.AttributeValueMemberN{Value: "10"},
		},
		{
			name:        "Ints/Int64/Min",
			goValue:     math.MinInt64,
			dynamoValue: &types.AttributeValueMemberN{Value: "-9223372036854775808"},
		},
		{
			name:        "Ints/Int64/Max",
			goValue:     math.MaxInt64,
			dynamoValue: &types.AttributeValueMemberN{Value: "9223372036854775807"},
		},
		{
			name:        "UInts/UInt8",
			goValue:     uint8(12),
			dynamoValue: &types.AttributeValueMemberN{Value: "12"},
		},
		{
			name:        "UInts/UInt8/Min",
			goValue:     uint8(0),
			dynamoValue: &types.AttributeValueMemberN{Value: "0"},
		},
		{
			name:        "UInts/UInt8/Max",
			goValue:     math.MaxUint8,
			dynamoValue: &types.AttributeValueMemberN{Value: "255"},
		},
		{
			name:        "UInts/UInt16",
			goValue:     uint16(12),
			dynamoValue: &types.AttributeValueMemberN{Value: "12"},
		},
		{
			name:        "UInts/UInt16/Min",
			goValue:     uint16(0),
			dynamoValue: &types.AttributeValueMemberN{Value: "0"},
		},
		{
			name:        "UInts/UInt16/Max",
			goValue:     math.MaxUint16,
			dynamoValue: &types.AttributeValueMemberN{Value: "65535"},
		},
		{
			name:        "UInts/UInt32",
			goValue:     uint32(12),
			dynamoValue: &types.AttributeValueMemberN{Value: "12"},
		},
		{
			name:        "UInts/UInt32/Min",
			goValue:     uint32(0),
			dynamoValue: &types.AttributeValueMemberN{Value: "0"},
		},
		{
			name:        "UInts/UInt32/Max",
			goValue:     math.MaxUint32,
			dynamoValue: &types.AttributeValueMemberN{Value: "4294967295"},
		},
		{
			name:        "UInts/UInt64",
			goValue:     uint64(12),
			dynamoValue: &types.AttributeValueMemberN{Value: "12"},
		},
		{
			name:        "UInts/UInt32/Min",
			goValue:     uint64(0),
			dynamoValue: &types.AttributeValueMemberN{Value: "0"},
		},
		{
			name:        "UInts/UInt64/Max",
			goValue:     uint64(math.MaxUint64),
			dynamoValue: &types.AttributeValueMemberN{Value: "18446744073709551615"},
		},
		{
			name:        "Floats/Float32",
			goValue:     float32(526.25),
			dynamoValue: &types.AttributeValueMemberN{Value: "526.25"},
		},
		{
			name:        "Floats/Float64",
			goValue:     float64(-12526.325),
			dynamoValue: &types.AttributeValueMemberN{Value: "-12526.325"},
		},
		{
			name:        "Floats/Float32",
			goValue:     float32(0.5),
			dynamoValue: &types.AttributeValueMemberN{Value: "0.5"},
		},
		{
			name:        "Floats/Float64",
			goValue:     float64(0.75),
			dynamoValue: &types.AttributeValueMemberN{Value: "0.75"},
		},
		{
			name:    "Bytes",
			goValue: [][]byte{nil, {}, {1}, {1, 2}, {1, 2, 3}},
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
			name:    "Bytes/Large",
			goValue: []byte("abcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()"),
			dynamoValue: &types.AttributeValueMemberB{
				Value: []byte("abcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()"),
			},
		},
		{
			name:        "Bytes/ByteArray",
			goValue:     [5]byte{'h', 'e', 'l', 'l', 'o'},
			dynamoValue: &types.AttributeValueMemberB{Value: []byte{'h', 'e', 'l', 'l', 'o'}},
		},
		{
			name:        "Bytes/NamedByteArray",
			goValue:     [5]namedByte{'h', 'e', 'l', 'l', 'o'},
			dynamoValue: &types.AttributeValueMemberB{Value: []byte{'h', 'e', 'l', 'l', 'o'}},
		},
		{
			name:        "Bytes/NamedByteArrayNotFull",
			goValue:     [5]namedByte{'h', 'e', 'l', 'l'},
			dynamoValue: &types.AttributeValueMemberB{Value: []byte{'h', 'e', 'l', 'l', 0}},
		},
		{
			name:        "Bytes/NamedByteSlice",
			goValue:     [5]namedByte{'h', 'e', 'l', 'l', 'o'},
			dynamoValue: &types.AttributeValueMemberB{Value: []byte{'h', 'e', 'l', 'l', 'o'}},
		},
		{
			name:        "Pointers/NilL0",
			goValue:     (*int)(nil),
			dynamoValue: &types.AttributeValueMemberNULL{Value: true},
		},
		{
			name:        "Pointers/NilL1",
			goValue:     new(*int),
			dynamoValue: &types.AttributeValueMemberNULL{Value: true},
		},
		{
			name:        "Pointers/Bool",
			goValue:     addr(addr(bool(true))),
			dynamoValue: &types.AttributeValueMemberBOOL{Value: true},
		},
		{
			name:        "Pointers/String",
			goValue:     addr(addr(string("string"))),
			dynamoValue: &types.AttributeValueMemberS{Value: "string"},
		},
		{
			name:        "Pointers/Bytes",
			goValue:     addr(addr([]byte("bytes"))),
			dynamoValue: &types.AttributeValueMemberB{Value: ([]byte)("bytes")},
		},
		{
			name:        "Pointers/Int",
			goValue:     addr(addr(int(-100))),
			dynamoValue: &types.AttributeValueMemberN{Value: "-100"},
		},
		{
			name:        "Pointers/Uint",
			goValue:     addr(addr(uint(100))),
			dynamoValue: &types.AttributeValueMemberN{Value: "100"},
		},
		{
			name:        "Pointers/Float64",
			goValue:     addr(addr(float64(3.14159))),
			dynamoValue: &types.AttributeValueMemberN{Value: "3.14159"},
		},
		{
			name:        "Pointers/Float324",
			goValue:     addr(float32(1.25)),
			dynamoValue: &types.AttributeValueMemberN{Value: "1.25"},
		},
		{
			name:    "Maps/String/String",
			goValue: map[string]int{"a": 0, "b": 1, "c": 2},
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
			goValue: map[bool]string{
				true:  "aaa",
				false: "bbb",
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"true":  &types.AttributeValueMemberS{Value: "aaa"},
					"false": &types.AttributeValueMemberS{Value: "bbb"},
				},
			},
		},
		{
			name:    "Maps/NamedBool/String",
			goValue: map[namedBool]string{false: "aaa"},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"false": &types.AttributeValueMemberS{Value: "aaa"},
				},
			},
		},
		{
			name:    "Maps/Int64/String",
			goValue: map[namedBool]string{false: "aaa"},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"false": &types.AttributeValueMemberS{Value: "aaa"},
				},
			},
		},
		{
			name:    "Maps/Int/String",
			goValue: map[int64]string{math.MinInt64: "MinInt64", 0: "Zero", math.MaxInt64: "MaxInt64"},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"-9223372036854775808": &types.AttributeValueMemberS{Value: "MinInt64"},
					"0":                    &types.AttributeValueMemberS{Value: "Zero"},
					"9223372036854775807":  &types.AttributeValueMemberS{Value: "MaxInt64"},
				},
			},
		},
		{
			name:    "Maps/Uint64/String",
			goValue: map[uint64]string{0: "Zero", math.MaxUint64: "MaxUint64"},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"0":                    &types.AttributeValueMemberS{Value: "Zero"},
					"18446744073709551615": &types.AttributeValueMemberS{Value: "MaxUint64"},
				},
			},
		},

		{
			name:    "Maps/NamedUint64/String",
			goValue: map[namedUint64]string{0: "Zero", math.MaxUint64: "MaxUint64"},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"0":                    &types.AttributeValueMemberS{Value: "Zero"},
					"18446744073709551615": &types.AttributeValueMemberS{Value: "MaxUint64"},
				},
			},
		},
		{
			name:    "Maps/NamedInt64/String",
			goValue: map[namedInt64]string{math.MinInt64: "MinInt64", 0: "Zero", math.MaxInt64: "MaxInt64"},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"-9223372036854775808": &types.AttributeValueMemberS{Value: "MinInt64"},
					"0":                    &types.AttributeValueMemberS{Value: "Zero"},
					"9223372036854775807":  &types.AttributeValueMemberS{Value: "MaxInt64"},
				},
			},
		},
		{
			name: "Maps/SkipEmptyKeys",
			goValue: map[string]string{
				"": "bb",
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{},
			},
		},
		{
			name: "Maps/TextMarshaler/String",
			goValue: map[lowerCasedString]string{
				lowerCasedString("aa"): "test1",
				lowerCasedString("XX"): "test2",
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"aa": &types.AttributeValueMemberS{Value: "test1"},
					"xx": &types.AttributeValueMemberS{Value: "test2"},
				},
			},
		},
		{
			name: "Maps/TextMarshaler/DuplicateKey",
			goValue: map[encoding.TextMarshaler]string{
				lowerCasedString("xx"): "test1",
				lowerCasedString("XX"): "tes2", //will cause duplicate key error
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[map[encoding.TextMarshaler]string](),
					Err:    fmt.Errorf("marshal key encountered duplicate key with value xx"),
				},
			),
		},
		{
			name: "Maps/TextMarshaler/ErrorMarshalingKey",
			goValue: map[encoding.TextMarshaler]string{
				TextMarshalerMock{Err: fmt.Errorf("mock-error")}: "a",
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[map[encoding.TextMarshaler]string](),
					Kind:   serializer.DynamoKindMap,
					Err: serializer.NewMarshalError(
						serializer.SerializerErrorData{
							GoType: reflect.TypeFor[encoding.TextMarshaler](),
							Err:    fmt.Errorf("mock-error"),
						},
					),
				},
			),
		},
		{
			name:    "Maps/InvalidKeyType/Array",
			goValue: map[[1]string]string{{"key"}: "value"},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[map[[1]string]string](),
					Kind:   serializer.DynamoKindMap,
					Err:    fmt.Errorf("map key of type [1]string is not supported"),
				},
			),
		},
		{
			name: "Maps/InValidKey/EmptyStruct",
			goValue: map[struct{}]string{
				{}: "a",
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[map[struct{}]string](),
					Kind:   serializer.DynamoKindMap,
					Err:    fmt.Errorf("map key of type struct {} is not supported"),
				},
			),
		},
		{
			name:    "Maps/InvalidKey/Channel",
			goValue: map[chan string]string{make(chan string): "value"},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[map[chan string]string](),
					Kind:   serializer.DynamoKindMap,
					Err:    fmt.Errorf("map key of type chan string is not supported"),
				},
			),
		},
		{
			name: "Maps/InValidKey/TextMarshalerMockPtr",
			goValue: map[TextMarshalerMockPtr]string{
				{Err: fmt.Errorf("mock-error")}: "a",
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[map[TextMarshalerMockPtr]string](),
					Kind:   serializer.DynamoKindMap,
					Err: serializer.NewMarshalError(
						serializer.SerializerErrorData{
							GoType: reflect.TypeFor[TextMarshalerMockPtr](),
							Err:    fmt.Errorf("mock-error"),
						},
					),
				},
			),
		},

		{
			name: "Maps/InValidKey/TextMarshaler",
			goValue: map[encoding.TextMarshaler]string{
				TextMarshalerMock{Err: fmt.Errorf("mock-error")}: "a",
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[map[encoding.TextMarshaler]string](),
					Kind:   serializer.DynamoKindMap,
					Err: serializer.NewMarshalError(
						serializer.SerializerErrorData{
							GoType: reflect.TypeFor[encoding.TextMarshaler](),
							Err:    fmt.Errorf("mock-error"),
						},
					),
				},
			),
		},
		{
			name: "Maps/UnsupportedValueType/Channel",
			goValue: map[string]chan string{
				"key": nil,
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Err: serializer.NewMarshalError(
						serializer.SerializerErrorData{
							Err:    errors.ErrUnsupported,
							GoType: reflect.TypeFor[chan string](),
							Kind:   serializer.DynamoKindUnknown,
						},
					),
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[map[string]chan string](),
				},
			),
		},
		{
			name: "Structs/Normal",
			goValue: structAll{
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
					MapBool:   map[string]bool{"test": true},
					MapString: map[string]string{"test": "hello"},
					MapBytes:  map[string][]byte{"test": {1, 2, 3}},
					MapInt:    map[string]int64{"test": -64},
					MapUint:   map[string]uint64{"test": +64},
					MapFloat:  map[string]float64{"test": 3.14159},
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
				Interface: (*structAll)(nil),
			},
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
							"key": &types.AttributeValueMemberS{Value: "value"},
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
							"MapBool": &types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{
									"test": &types.AttributeValueMemberBOOL{Value: true},
								},
							},
							"MapString": &types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{
									"test": &types.AttributeValueMemberS{Value: "hello"},
								},
							},
							"MapBytes": &types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{
									"test": &types.AttributeValueMemberB{Value: []byte{1, 2, 3}},
								},
							},
							"MapInt": &types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{
									"test": &types.AttributeValueMemberN{Value: "-64"},
								},
							},
							"MapUint": &types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{
									"test": &types.AttributeValueMemberN{Value: "64"},
								},
							},
							"MapFloat": &types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{
									"test": &types.AttributeValueMemberN{Value: "3.14159"},
								},
							},
						},
					},
					"StructSlices": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"SliceBool": &types.AttributeValueMemberL{
								Value: []types.AttributeValue{
									&types.AttributeValueMemberBOOL{Value: true},
								},
							},
							"SliceString": &types.AttributeValueMemberL{
								Value: []types.AttributeValue{
									&types.AttributeValueMemberS{Value: "hello"},
								},
							},
							"SliceBytes": &types.AttributeValueMemberL{
								Value: []types.AttributeValue{
									&types.AttributeValueMemberB{Value: []byte{1, 2, 3}},
								},
							},
							"SliceInt": &types.AttributeValueMemberL{
								Value: []types.AttributeValue{
									&types.AttributeValueMemberN{Value: "-64"},
								},
							},
							"SliceUint": &types.AttributeValueMemberL{
								Value: []types.AttributeValue{
									&types.AttributeValueMemberN{Value: "64"},
								},
							},
							"SliceFloat": &types.AttributeValueMemberL{
								Value: []types.AttributeValue{
									&types.AttributeValueMemberN{Value: "3.14159"},
								},
							},
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
						Value: map[string]types.AttributeValue{
							"Bool":   &types.AttributeValueMemberBOOL{Value: false},
							"String": &types.AttributeValueMemberS{Value: ""},
							"Bytes":  &types.AttributeValueMemberB{Value: nil},
							"Int":    &types.AttributeValueMemberN{Value: "0"},
							"Uint":   &types.AttributeValueMemberN{Value: "0"},
							"Float":  &types.AttributeValueMemberN{Value: "0"},
							"Map":    &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
							"StructScalars": &types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{
									"Bool":   &types.AttributeValueMemberBOOL{Value: false},
									"String": &types.AttributeValueMemberS{Value: ""},
									"Bytes":  &types.AttributeValueMemberB{Value: nil},
									"Int":    &types.AttributeValueMemberN{Value: "0"},
									"Uint":   &types.AttributeValueMemberN{Value: "0"},
									"Float":  &types.AttributeValueMemberN{Value: "0"},
								},
							},
							"StructMaps": &types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{
									"MapBool":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
									"MapString": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
									"MapBytes":  &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
									"MapInt":    &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
									"MapUint":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
									"MapFloat":  &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
								},
							},
							"StructSlices": &types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{
									"SliceBool":   &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
									"SliceString": &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
									"SliceBytes":  &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
									"SliceInt":    &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
									"SliceUint":   &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
									"SliceFloat":  &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
								},
							},
							"Slice": &types.AttributeValueMemberL{
								Value: []types.AttributeValue{},
							},
							"Array": &types.AttributeValueMemberL{
								Value: []types.AttributeValue{
									&types.AttributeValueMemberS{Value: ""},
								},
							},
							"Pointer":   &types.AttributeValueMemberNULL{Value: true},
							"Interface": &types.AttributeValueMemberNULL{Value: true},
						},
					},
					"Interface": &types.AttributeValueMemberNULL{Value: true},
				},
			},
		},
		{
			name:        "Structs/Empty",
			goValue:     struct{}{},
			dynamoValue: &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
		},
		{
			name:        "Structs/NamedEmptyStruct",
			goValue:     namedEmptyStruct{},
			dynamoValue: &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
		},
		{
			name: "Structs/CustomTag",
			goValue: struct {
				X int `test_tag:"custom_tag"`
			}{
				X: 100,
			},
			opts: []serializer.EncoderOption{
				serializer.WithTag("test_tag"),
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"custom_tag": &types.AttributeValueMemberN{Value: "100"},
				},
			},
		},
		{
			name: "Structs/OmitEmpty/NamedEmptyObject",
			goValue: struct {
				EmptyObject namedEmptyStruct `dynamodbav:",omitempty"`
			}{},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{},
			},
		},
		{
			name: "Structs/OmitEmpty/NonEmpty",
			goValue: structOmitEmptyAll{
				Bool:                  true,
				PointerBool:           addr(true),
				String:                string("value"),
				StringEmpty:           stringMarshalEmpty("value"),
				StringNonEmpty:        stringMarshalNonEmpty("value"),
				PointerString:         addr(string("value")),
				PointerStringEmpty:    addr(stringMarshalEmpty("value")),
				PointerStringNonEmpty: addr(stringMarshalNonEmpty("value")),
				Bytes:                 []byte("value"),
				BytesEmpty:            bytesMarshalEmpty([]byte("value")),
				BytesNonEmpty:         bytesMarshalNonEmpty([]byte("value")),
				PointerBytes:          addr([]byte("value")),
				PointerBytesEmpty:     addr(bytesMarshalEmpty([]byte("value"))),
				PointerBytesNonEmpty:  addr(bytesMarshalNonEmpty([]byte("value"))),
				Float:                 math.Copysign(0, -1),
				PointerFloat:          addr(math.Copysign(0, -1)),
				Map:                   map[string]string{"test": ""},
				MapEmpty:              mapMarshalEmpty{"key": "value"},
				MapNonEmpty:           mapMarshalNonEmpty{"key": "value"},
				PointerMap:            addr(map[string]string{"test": ""}),
				PointerMapEmpty:       addr(mapMarshalEmpty{"key": "value"}),
				PointerMapNonEmpty:    addr(mapMarshalNonEmpty{"key": "value"}),
				Slice:                 []string{""},
				SliceEmpty:            sliceMarshalEmpty{"value"},
				SliceNonEmpty:         sliceMarshalNonEmpty{"value"},
				PointerSlice:          addr([]string{""}),
				PointerSliceEmpty:     addr(sliceMarshalEmpty{"value"}),
				PointerSliceNonEmpty:  addr(sliceMarshalNonEmpty{"value"}),
				Pointer:               &structOmitZeroEmptyAll{Float: math.SmallestNonzeroFloat64},
				Interface:             []string{""},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Bool":                  &types.AttributeValueMemberBOOL{Value: true},
					"PointerBool":           &types.AttributeValueMemberBOOL{Value: true},
					"String":                &types.AttributeValueMemberS{Value: "value"},
					"StringNonEmpty":        &types.AttributeValueMemberS{Value: "value"},
					"PointerString":         &types.AttributeValueMemberS{Value: "value"},
					"PointerStringNonEmpty": &types.AttributeValueMemberS{Value: "value"},
					"Bytes":                 &types.AttributeValueMemberB{Value: []byte("value")},
					"BytesNonEmpty":         &types.AttributeValueMemberB{Value: []byte("value")},
					"PointerBytes":          &types.AttributeValueMemberB{Value: []byte("value")},
					"PointerBytesNonEmpty":  &types.AttributeValueMemberB{Value: []byte("value")},
					"Float":                 &types.AttributeValueMemberN{Value: "-0"},
					"PointerFloat":          &types.AttributeValueMemberN{Value: "-0"},
					"Map": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"test": &types.AttributeValueMemberS{Value: ""},
						},
					},
					"MapNonEmpty": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"key": &types.AttributeValueMemberS{Value: "value"},
						},
					},
					"PointerMap": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"test": &types.AttributeValueMemberS{Value: ""},
						},
					},
					"PointerMapNonEmpty": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"key": &types.AttributeValueMemberS{Value: "value"},
						},
					},
					"Slice": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberS{Value: ""},
						},
					},
					"SliceNonEmpty": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberS{Value: "value"},
						},
					},
					"PointerSlice": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberS{Value: ""},
						},
					},
					"PointerSliceNonEmpty": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberS{Value: "value"},
						},
					},
					"Pointer": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
						"Float": &types.AttributeValueMemberN{Value: "5e-324"},
					}},
					"Interface": &types.AttributeValueMemberL{Value: []types.AttributeValue{
						&types.AttributeValueMemberS{Value: ""},
					}},
				},
			},
		},
		{
			name: "Structs/OmitEmpty/Recursive",
			goValue: func() any {
				type X struct {
					X *X `dynamodbav:",omitempty"`
				}
				var make func(int) *X
				make = func(n int) *X {
					if n == 0 {
						return nil
					}
					return &X{make(n - 1)}
				}
				return make(100)
			}(),
			dynamoValue: &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
		},
		{
			name: "Structs/OmitEmpty/EmptyString",
			goValue: struct {
				Str string `dynamodbav:",omitempty"`
			}{
				Str: "",
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{},
			},
		},
		{
			name: "Structs/OmitEmpty/NonEmptyString",
			goValue: struct {
				Str string `dynamodbav:",omitempty"`
			}{
				Str: `"`,
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Str": &types.AttributeValueMemberS{Value: `"`},
				},
			},
		},
		{
			name: "Structs/Maps/Empty",
			goValue: struct {
				Map map[string]string `dynamodbav:",omitempty"`
			}{
				Map: map[string]string{},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{},
			},
		},
		{
			name: "Structs/Maps/OmitZero",
			goValue: struct {
				Map map[string]structAll `dynamodbav:",omitzero"`
			}{},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{},
			},
		},
		{
			name:        "Structs/OmitZeroEmpty/Zero",
			goValue:     structOmitZeroEmptyAll{},
			dynamoValue: &types.AttributeValueMemberM{Value: make(map[string]types.AttributeValue, 0)},
		},
		{
			name: "Structs/OmitZeroEmpty/Empty",
			goValue: structOmitZeroEmptyAll{
				Bytes:     []byte{},
				Map:       map[string]string{},
				Slice:     []string{},
				Pointer:   &structOmitZeroEmptyAll{},
				Interface: []string{},
			},
			dynamoValue: &types.AttributeValueMemberM{Value: make(map[string]types.AttributeValue, 0)},
		},
		{
			name: "Structs/OmitEmpty/EmptyNonZero",
			goValue: structOmitEmptyAll{
				Bool:                  false,
				PointerBool:           addr(false),
				Float:                 0.0,
				PointerFloat:          addr(0.0),
				String:                string(""),
				StringEmpty:           stringMarshalEmpty(""),
				StringNonEmpty:        stringMarshalNonEmpty(""),
				PointerString:         addr(string("")),
				PointerStringEmpty:    addr(stringMarshalEmpty("")),
				PointerStringNonEmpty: addr(stringMarshalNonEmpty("")),
				Bytes:                 []byte(""),
				BytesEmpty:            bytesMarshalEmpty([]byte("abc")),
				BytesNonEmpty:         bytesMarshalNonEmpty([]byte("xyz")),
				PointerBytes:          addr([]byte("")),
				PointerBytesEmpty:     addr(bytesMarshalEmpty([]byte(""))),
				PointerBytesNonEmpty:  addr(bytesMarshalNonEmpty([]byte(""))),
				Map:                   map[string]string{},
				MapEmpty:              mapMarshalEmpty{},
				MapNonEmpty:           mapMarshalNonEmpty{},
				PointerMap:            addr(map[string]string{}),
				PointerMapEmpty:       addr(mapMarshalEmpty{}),
				PointerMapNonEmpty:    addr(mapMarshalNonEmpty{}),
				Slice:                 []string{},
				SliceEmpty:            sliceMarshalEmpty{},
				SliceNonEmpty:         sliceMarshalNonEmpty{},
				PointerSlice:          addr([]string{}),
				PointerSliceEmpty:     addr(sliceMarshalEmpty{}),
				PointerSliceNonEmpty:  addr(sliceMarshalNonEmpty{}),
				Pointer:               &structOmitZeroEmptyAll{},
				Interface:             []string{},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Bool":                  &types.AttributeValueMemberBOOL{Value: false},
					"PointerBool":           &types.AttributeValueMemberBOOL{Value: false},
					"StringNonEmpty":        &types.AttributeValueMemberS{Value: "value"},
					"Float":                 &types.AttributeValueMemberN{Value: "0"},
					"PointerFloat":          &types.AttributeValueMemberN{Value: "0"},
					"PointerStringNonEmpty": &types.AttributeValueMemberS{Value: "value"},
					"BytesNonEmpty":         &types.AttributeValueMemberB{Value: []byte("value")},
					"PointerBytesNonEmpty":  &types.AttributeValueMemberB{Value: []byte("value")},
					"MapNonEmpty":           &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"key": &types.AttributeValueMemberS{Value: "value"}}},
					"PointerMapNonEmpty":    &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"key": &types.AttributeValueMemberS{Value: "value"}}},
					"SliceNonEmpty":         &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberS{Value: "value"}}},
					"PointerSliceNonEmpty":  &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberS{Value: "value"}}},
				},
			},
		},
		{
			name: "Structs/OmitZeroEmpty/NonEmpty",
			goValue: structOmitZeroEmptyAll{
				Bytes:     []byte("value"),
				Map:       map[string]string{"test": ""},
				Slice:     []string{""},
				Pointer:   &structOmitZeroEmptyAll{Bool: true},
				Interface: []string{""},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Bytes": &types.AttributeValueMemberB{Value: []byte("value")},
					"Map": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"test": &types.AttributeValueMemberS{Value: ""},
						},
					},
					"Slice": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberS{Value: ""},
						},
					},
					"Pointer": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"Bool": &types.AttributeValueMemberBOOL{Value: true},
						},
					},
					"Interface": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberS{Value: ""},
						},
					},
				},
			},
		},
		{
			name: "Structs/OmitEmpty/EmptyObject",
			goValue: struct {
				EmptyObject struct{} `dynamodbav:",omitempty"`
			}{},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{},
			},
		},
		{
			name:        "Structs/OmitZero/Zero",
			goValue:     structOmitZeroAll{},
			dynamoValue: &types.AttributeValueMemberM{Value: make(map[string]types.AttributeValue)},
		},
		{
			name: "Structs/OmitZero/NonZero",
			goValue: structOmitZeroAll{
				Bool:          true,                                   // not omitted since true is non-zero
				String:        " ",                                    // not omitted since non-empty string is non-zero
				Bytes:         []byte{},                               // not omitted since allocated slice is non-zero
				Int:           1,                                      // not omitted since 1 is non-zero
				Uint:          1,                                      // not omitted since 1 is non-zero
				Float:         math.SmallestNonzeroFloat64,            // not omitted since still slightly above zero
				Map:           map[string]string{},                    // not omitted since allocated map is non-zero
				StructScalars: structScalars{unexported: true},        // not omitted since unexported is non-zero
				StructSlices:  structSlices{Ignored: true},            // not omitted since Ignored is non-zero
				StructMaps:    structMaps{MapBool: map[string]bool{}}, // not omitted since MapBool is non-zero
				Slice:         []string{},                             // not omitted since allocated slice is non-zero
				Array:         [1]string{" "},                         // not omitted since single array element is non-zero
				Pointer:       new(structOmitZeroAll),                 // not omitted since pointer is non-zero (even if all fields of the struct value are zero)
				Interface:     (*structOmitZeroAll)(nil),              // not omitted since interface value is non-zero (even if interface value is a nil pointer)
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Bool":   &types.AttributeValueMemberBOOL{Value: true},
					"String": &types.AttributeValueMemberS{Value: " "},
					"Bytes":  &types.AttributeValueMemberB{Value: []byte("")},
					"Int":    &types.AttributeValueMemberN{Value: "1"},
					"Uint":   &types.AttributeValueMemberN{Value: "1"},
					"Float":  &types.AttributeValueMemberN{Value: "5e-324"},
					"Map":    &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
					"StructScalars": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
						"Bool":   &types.AttributeValueMemberBOOL{Value: false},
						"String": &types.AttributeValueMemberS{Value: ""},
						"Bytes":  &types.AttributeValueMemberB{Value: nil},
						"Int":    &types.AttributeValueMemberN{Value: "0"},
						"Uint":   &types.AttributeValueMemberN{Value: "0"},
						"Float":  &types.AttributeValueMemberN{Value: "0"},
					}},
					"StructMaps": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"MapBool":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
							"MapString": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
							"MapBytes":  &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
							"MapInt":    &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
							"MapUint":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
							"MapFloat":  &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
						},
					},
					"StructSlices": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"SliceBool":   &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
							"SliceString": &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
							"SliceBytes":  &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
							"SliceInt":    &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
							"SliceUint":   &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
							"SliceFloat":  &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
						},
					},
					"Slice": &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
					"Array": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberS{Value: " "},
						},
					},
					"Pointer":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
					"Interface": &types.AttributeValueMemberNULL{Value: true},
				},
			},
		},
		{
			name:        "structs/OmitZeroStructElements",
			goValue:     structAll{},
			opts:        []serializer.EncoderOption{serializer.OmitZeroStructFields()},
			dynamoValue: &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
		},
		{
			name:        "Structs/OmitZeroMethod/Interface/Zero",
			goValue:     structOmitZeroMethodInterfaceAll{},
			dynamoValue: &types.AttributeValueMemberM{Value: make(map[string]types.AttributeValue, 0)},
		},
		{
			name: "Structs/OmitZeroMethod/Interface/PartialZero",
			goValue: structOmitZeroMethodInterfaceAll{
				ValueAlwaysZero:          valueAlwaysZero(""),
				ValueNeverZero:           valueNeverZero(""),
				PointerValueAlwaysZero:   (*valueAlwaysZero)(nil),
				PointerValueNeverZero:    (*valueNeverZero)(nil), // nil pointer, so method not called
				PointerPointerAlwaysZero: (*pointerAlwaysZero)(nil),
				PointerPointerNeverZero:  (*pointerNeverZero)(nil), // nil pointer, so method not called
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"ValueNeverZero": &types.AttributeValueMemberS{Value: ""},
				},
			},
		},
		{
			name:    "Structs/OmitZeroMethod/Zero",
			goValue: structOmitZeroMethodAll{},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"ValueNeverZero":   &types.AttributeValueMemberS{Value: ""},
					"PointerNeverZero": &types.AttributeValueMemberS{Value: ""},
				},
			},
		},
		{
			name: "Structs/OmitZeroMethod/NonZero",
			goValue: structOmitZeroMethodAll{
				ValueAlwaysZero:                 valueAlwaysZero("nonzero"),
				ValueNeverZero:                  valueNeverZero("nonzero"),
				PointerAlwaysZero:               pointerAlwaysZero("nonzero"),
				PointerNeverZero:                pointerNeverZero("nonzero"),
				PointerValueAlwaysZero:          addr(valueAlwaysZero("nonzero")),
				PointerValueNeverZero:           addr(valueNeverZero("nonzero")),
				PointerPointerAlwaysZero:        addr(pointerAlwaysZero("nonzero")),
				PointerPointerNeverZero:         addr(pointerNeverZero("nonzero")),
				PointerPointerValueAlwaysZero:   addr(addr(valueAlwaysZero("nonzero"))), // marshaled since **valueAlwaysZero does not implement IsZero
				PointerPointerValueNeverZero:    addr(addr(valueNeverZero("nonzero"))),
				PointerPointerPointerAlwaysZero: addr(addr(pointerAlwaysZero("nonzero"))), // marshaled since **pointerAlwaysZero does not implement IsZero
				PointerPointerPointerNeverZero:  addr(addr(pointerNeverZero("nonzero"))),
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"ValueNeverZero":                  &types.AttributeValueMemberS{Value: "nonzero"},
					"PointerNeverZero":                &types.AttributeValueMemberS{Value: "nonzero"},
					"PointerValueNeverZero":           &types.AttributeValueMemberS{Value: "nonzero"},
					"PointerPointerNeverZero":         &types.AttributeValueMemberS{Value: "nonzero"},
					"PointerPointerValueAlwaysZero":   &types.AttributeValueMemberS{Value: "nonzero"},
					"PointerPointerValueNeverZero":    &types.AttributeValueMemberS{Value: "nonzero"},
					"PointerPointerPointerAlwaysZero": &types.AttributeValueMemberS{Value: "nonzero"},
					"PointerPointerPointerNeverZero":  &types.AttributeValueMemberS{Value: "nonzero"},
				},
			},
		},
		{
			name: "Structs/OmitZeroMethod/Interface/NonZero",
			goValue: structOmitZeroMethodInterfaceAll{
				ValueAlwaysZero:          valueAlwaysZero("nonzero"),
				ValueNeverZero:           valueNeverZero("nonzero"),
				PointerValueAlwaysZero:   addr(valueAlwaysZero("nonzero")),
				PointerValueNeverZero:    addr(valueNeverZero("nonzero")),
				PointerPointerAlwaysZero: addr(pointerAlwaysZero("nonzero")),
				PointerPointerNeverZero:  addr(pointerNeverZero("nonzero")),
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"ValueNeverZero":          &types.AttributeValueMemberS{Value: "nonzero"},
					"PointerValueNeverZero":   &types.AttributeValueMemberS{Value: "nonzero"},
					"PointerPointerNeverZero": &types.AttributeValueMemberS{Value: "nonzero"},
				},
			},
		},
		{
			name:    "Structs/Invalid/Conflicting",
			goValue: structConflicting{},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[structConflicting](),
					Kind:   serializer.DynamoKindMap,
					Err:    fmt.Errorf("Go struct fields A and B conflict over dynamodbav object name \"conflict\""),
				},
			),
		},
		{
			name: "Structs/Inline/DualCycle",
			goValue: cyclicA{
				B1: cyclicB{F: 1}, // B1.F ignored since it conflicts with B2.F
				B2: cyclicB{F: 2}, // B2.F ignored since it conflicts with B1.F
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{},
			},
		},
		{
			name: "Structs/OmitEmpty/Null",
			goValue: struct {
				NullEmpty MockDynamoMarshaler `dynamodbav:",omitempty"`
			}{
				NullEmpty: MockDynamoMarshaler{},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{},
			},
		},
		{
			name:    "Structs/OmitEmpty/Zero",
			goValue: structOmitEmptyAll{},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Bool":           &types.AttributeValueMemberBOOL{Value: false},
					"StringNonEmpty": &types.AttributeValueMemberS{Value: "value"},
					"Float":          &types.AttributeValueMemberN{Value: "0"},
					"SliceNonEmpty":  &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberS{Value: "value"}}},
					"MapNonEmpty":    &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"key": &types.AttributeValueMemberS{Value: "value"}}},
					"BytesNonEmpty":  &types.AttributeValueMemberB{Value: []byte("value")},
				},
			},
		},
		{
			name: "Structs/Slices/OmitEmpty",
			goValue: struct {
				IntList     []int `dynamodbav:",omitempty"`
				IntListNull []int `dynamodbav:",omitempty"`
			}{
				IntList:     make([]int, 0),
				IntListNull: nil,
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{},
			},
		},
		{
			name: "Structs/BinaryEncoder",
			goValue: struct {
				binaryMarshalerMock
			}{binaryMarshalerMock: binaryMarshalerMock{
				Value: "hello",
			}},
			dynamoValue: &types.AttributeValueMemberB{Value: []byte("hello")},
		},
		{
			name: "Structs/BinaryEncoder/Error",
			goValue: struct {
				binaryMarshalerMock
			}{binaryMarshalerMock: binaryMarshalerMock{
				Err: fmt.Errorf("mock-error"),
			},
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[struct {
						binaryMarshalerMock
					}](),
					Err: fmt.Errorf("mock-error"),
				}),
		},
		{
			name: "Structs/TextMarshaler",
			goValue: struct {
				TextMarshalerMock
			}{TextMarshalerMock: TextMarshalerMock{
				Value: "hello",
			}},
			dynamoValue: &types.AttributeValueMemberS{Value: "hello"},
		},
		{
			name: "Structs/TextMarshaler/Error",
			goValue: struct {
				TextMarshalerMock
			}{TextMarshalerMock: TextMarshalerMock{
				Err: fmt.Errorf("mock-error"),
			},
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					GoType: reflect.TypeFor[struct {
						TextMarshalerMock
					}](),
					Err: fmt.Errorf("mock-error"),
				}),
		},
		{
			name: "Structs/Slices/OmitEmpty#2",
			goValue: struct {
				ListStr  []string `dynamodbav:",omitempty"`
				ListStr2 []string `dynamodbav:",omitempty"`
			}{
				ListStr:  make([]string, 0),
				ListStr2: []string{"A", "B", "C"},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"ListStr2": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberS{Value: "A"},
							&types.AttributeValueMemberS{Value: "B"},
							&types.AttributeValueMemberS{Value: "C"},
						},
					},
				},
			},
		},
		{
			name: "Structs/Ignored",
			goValue: struct {
				Skipped1    string `dynamodbav:"-"`
				Skipped2    string `dynamodbav:"-"`
				NotSkipped3 string `dynamodbav:"-,"` //not exact match to -
			}{
				Skipped2:    "abc",
				NotSkipped3: "aaa",
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"-": &types.AttributeValueMemberS{Value: "aaa"},
				},
			},
		},
		{
			name:        "Structs/UnexportedIgnored",
			goValue:     structUnexportedIgnored{ignored: "ignored"},
			dynamoValue: &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
		},
		{
			name:        "Structs/IgnoredUnexportedEmbedded",
			goValue:     structIgnoredUnexportedEmbedded{unexportedEmbeddedStruct: unexportedEmbeddedStruct{X: "ignored", y: "unexported"}},
			dynamoValue: &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
		},
		{
			name:        "Structs/Invalid/UnexportedEmbedded",
			goValue:     structUnexportedEmbedded{},
			dynamoValue: nil,
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[structUnexportedEmbedded](),
					Err:    fmt.Errorf("embedded Go struct field namedString of an unexported type must be explicitly ignored with a `dynamo:\"-\"` tag"),
				},
			),
		},
		{
			name:    "Structs/Invalid/StructUnexportedWithTag",
			goValue: structUnexportedTag{unexported: "unexported"},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[structUnexportedTag](),
					Err:    fmt.Errorf("unexported Go struct field unexported cannot have non-ignored `dynamo:\"new_name\"` tag , either remove the tag or put it equal to \"-\""),
				},
			),
		},
		{
			name: "Structs/Invalid/InlineNotStruct",
			goValue: struct {
				Int int64 `dynamodbav:",inline"`
			}{},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Kind: serializer.DynamoKindMap,
					GoType: reflect.TypeFor[struct {
						Int int64 `dynamodbav:",inline"`
					}](),
					Err: fmt.Errorf("inlined Go struct field Int of type int64 must be a Go struct"),
				},
			),
		},
		{
			name:    "Structs/IgnoreUnexportedUntagged",
			goValue: structIgnoreUnexportedUntagged{x: 12, Y: 21},
			dynamoValue: &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"Y": &types.AttributeValueMemberN{Value: "21"},
			}},
			expectedError: nil,
		},
		{
			name:    "Structs/Invalid/DuplicateFieldTag",
			goValue: DuplicateSetTags{Set: []string{}},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[DuplicateSetTags](),
					Err:    fmt.Errorf("struct field Set has duplicate occurrence of option set"),
				},
			),
		},
		{
			name: "Structs/Invalid/UnknownFlagCasing",
			goValue: struct {
				X []int `dynamodbav:",SEt"`
			}{X: []int{10, 20}},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Kind: serializer.DynamoKindMap,
					GoType: reflect.TypeFor[struct {
						X []int `dynamodbav:",SEt"`
					}](),
					Err: fmt.Errorf("struct field X has invalid appearance of `SEt` tag option; specify `set` instead"),
				},
			),
		},
		{
			name:    "Structs/Invalid/FieldsNotExported",
			goValue: StructNoExportedFields{},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[StructNoExportedFields](),
					Err:    fmt.Errorf("Go struct serializer_test.StructNoExportedFields has no exported fields"),
				},
			),
		},
		{
			name:    "Structs/Inline/Zero",
			goValue: structInlined{},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"D": &types.AttributeValueMemberS{Value: ""},
				},
			},
		},
		{
			name: "Structs/Inline/Alloc",
			goValue: structInlined{
				X: structInlinedL1{
					X:            &structInlinedL2{},
					StructEmbed1: StructEmbed1{},
				},
				StructEmbed2: &StructEmbed2{},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"A": &types.AttributeValueMemberS{Value: ""},
					"B": &types.AttributeValueMemberS{Value: ""},
					"D": &types.AttributeValueMemberS{Value: ""},
					"E": &types.AttributeValueMemberS{Value: ""},
					"F": &types.AttributeValueMemberS{Value: ""},
					"G": &types.AttributeValueMemberS{Value: ""},
				},
			},
		},
		{
			name: "Structs/Inline/NonZero",
			goValue: structInlined{
				X: structInlinedL1{
					X:            &structInlinedL2{A: "A1", B: "B1", C: "C1"},
					StructEmbed1: StructEmbed1{C: "C2", D: "D2", E: "E2"},
				},
				StructEmbed2: &StructEmbed2{E: "E3", F: "F3", G: "G3"},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"A": &types.AttributeValueMemberS{Value: "A1"},
					"B": &types.AttributeValueMemberS{Value: "B1"},
					"D": &types.AttributeValueMemberS{Value: "D2"},
					"E": &types.AttributeValueMemberS{Value: "E3"},
					"F": &types.AttributeValueMemberS{Value: "F3"},
					"G": &types.AttributeValueMemberS{Value: "G3"},
				},
			},
		},
		{
			name: "Structs/Inline/PreferNamed",
			goValue: struct {
				X             StructEmbed1Named `dynamodbav:",inline"` //explicit inline
				*StructEmbed2                   // implicit inline
			}{
				X: StructEmbed1Named{
					C: "C",
					D: "D",
					E: "E1",
				},
				StructEmbed2: &StructEmbed2{
					E: "E2",
					F: "F",
					G: "G",
				},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"C": &types.AttributeValueMemberS{Value: "C"},
					"D": &types.AttributeValueMemberS{Value: "D"},
					"E": &types.AttributeValueMemberS{Value: "E1"},
					"F": &types.AttributeValueMemberS{Value: "F"},
					"G": &types.AttributeValueMemberS{Value: "G"},
				},
			},
		},
		{
			name: "Structs/Inline/PreferNamed/Reversed",
			goValue: struct {
				StructEmbed1       // implicit inline
				*StructEmbed2Named // implicit inline
			}{
				StructEmbed1: StructEmbed1{
					C: "C",
					D: "D",
					E: "E1",
				},
				StructEmbed2Named: &StructEmbed2Named{
					E: "E2",
					F: "F",
					G: "G",
				},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"C": &types.AttributeValueMemberS{Value: "C"},
					"D": &types.AttributeValueMemberS{Value: "D"},
					"E": &types.AttributeValueMemberS{Value: "E2"},
					"F": &types.AttributeValueMemberS{Value: "F"},
					"G": &types.AttributeValueMemberS{Value: "G"},
				},
			},
		},
		{
			name: "Structs/EmbeddedValues",
			goValue: s1ValueEmbedded{
				Embedded1Val: Embedded1Val{
					X: 10,
				},
				Y: "B",
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberN{Value: "10"},
					"Y": &types.AttributeValueMemberS{Value: "B"},
				},
			},
		},
		{
			name: "Structs/Invalid/EmbeddedMockMarshaler",
			goValue: embeddedMockMarshaler{
				MockAvMarshaler: MockDynamoMarshaler{
					Value: &types.AttributeValueMemberBOOL{Value: true},
				},
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[embeddedMockMarshaler](),
					Err:    fmt.Errorf("inline field MockAvMarshaler of type serializer_test.MockDynamoMarshaler must not implement custom marshal or unmarshal"),
				},
			),
		},
		{
			name: "Structs/ConflictEmbeddedValueHidden",
			goValue: s2ValueEmbedded{
				Embedded1Val: Embedded1Val{
					X: 10,
				},
				X: true,
				Y: "BC",
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberBOOL{Value: true},
					"Y": &types.AttributeValueMemberS{Value: "BC"},
				},
			},
		},
		{
			name:    "Structs/EmbeddedValuePtr/Zero",
			goValue: sPtrValueEmbedded{},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Z": &types.AttributeValueMemberBOOL{Value: false},
				},
			},
		},
		{
			name:    "Structs/EmbeddedValuePtr/Nil",
			goValue: sPtrValueEmbedded{},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Z": &types.AttributeValueMemberBOOL{Value: false},
				},
			},
		},
		{
			name: "Structs/EmbeddedPtr",
			goValue: sPtrValueEmbedded{
				Embedded1Val: &Embedded1Val{X: 10},
				Z:            true,
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberN{Value: "10"},
					"Z": &types.AttributeValueMemberBOOL{Value: true},
				},
			},
		},
		{
			name:    "Structs/CustomMarshaler",
			goValue: structAll{},
			opts: []serializer.EncoderOption{
				serializer.WithMarshalers(
					serializer.MarshalFuncV1(
						func(structAll) (types.AttributeValue, error) {
							return &types.AttributeValueMemberBOOL{Value: true}, nil
						},
					),
				),
			},
			dynamoValue: &types.AttributeValueMemberBOOL{Value: true},
		},
		{
			name:    "Structs/CustomMarshaler/Error",
			goValue: structAll{},
			opts: []serializer.EncoderOption{
				serializer.WithMarshalers(
					serializer.MarshalFuncV1(
						func(structAll) (types.AttributeValue, error) {
							return nil, fmt.Errorf("mock-error")
						},
					),
				),
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindUnknown,
					GoType: reflect.TypeFor[structAll](),
					Err:    fmt.Errorf("mock-error"),
				},
			),
		},
		{
			name: "Interfaces/Any/Slice/Ints",
			goValue: []any{
				int(0), int8(math.MinInt8), int16(math.MinInt16), int32(math.MinInt32), int64(math.MinInt64), namedInt64(-6464),
			},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberN{Value: "0"},
					&types.AttributeValueMemberN{Value: "-128"},
					&types.AttributeValueMemberN{Value: "-32768"},
					&types.AttributeValueMemberN{Value: "-2147483648"},
					&types.AttributeValueMemberN{Value: "-9223372036854775808"},
					&types.AttributeValueMemberN{Value: "-6464"},
				},
			},
		},
		{
			name: "Interfaces/Any/Slice/Floats",
			goValue: []any{
				float32(math.MaxFloat32),
				float64(math.MaxFloat64),
				namedFloat64(64.64),
			},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberN{Value: strconv.FormatFloat(float64(math.MaxFloat32), 'g', -1, 32)},
					&types.AttributeValueMemberN{Value: strconv.FormatFloat(float64(math.MaxFloat64), 'g', -1, 64)},
					&types.AttributeValueMemberN{Value: "64.64"},
				},
			},
		},
		{
			name: "Interfaces/Struct/Any",
			goValue: struct{ X any }{
				[]any{
					nil,
					false,
					"",
					0.0,
					map[string]any{},
					[]any{},
					[8]byte{},
					[]byte{'a', 'b'},
					uint8(math.MaxUint8),
					uint16(math.MaxUint16),
					uint32(math.MaxUint32),
					uint64(math.MaxUint64),
					uint(math.MaxUint),
				},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberL{Value: []types.AttributeValue{
						&types.AttributeValueMemberNULL{Value: true},
						&types.AttributeValueMemberBOOL{Value: false},
						&types.AttributeValueMemberS{Value: ""},
						&types.AttributeValueMemberN{Value: "0"},
						&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
						&types.AttributeValueMemberL{Value: []types.AttributeValue{}},
						&types.AttributeValueMemberB{Value: []byte{0, 0, 0, 0, 0, 0, 0, 0}},
						&types.AttributeValueMemberB{Value: []byte{'a', 'b'}},
						&types.AttributeValueMemberN{Value: "255"},
						&types.AttributeValueMemberN{Value: "65535"},
						&types.AttributeValueMemberN{Value: "4294967295"},
						&types.AttributeValueMemberN{Value: "18446744073709551615"},
						&types.AttributeValueMemberN{Value: strconv.FormatUint(math.MaxUint, 10)},
					},
					},
				},
			},
		},
		{
			name: "Interfaces/Any/Map",
			goValue: map[string]any{
				"X": map[string]any{
					"Y": 20,
					"Z": true,
					"Set": struct {
						X []string `dynamodbav:",set"`
					}{
						X: []string{"a", "b", "c"},
					},
				},
				"B": false,
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"Y": &types.AttributeValueMemberN{Value: "20"},
							"Z": &types.AttributeValueMemberBOOL{Value: true},
							"Set": &types.AttributeValueMemberM{
								Value: map[string]types.AttributeValue{
									"X": &types.AttributeValueMemberSS{
										Value: []string{"a", "b", "c"},
									},
								},
							},
						},
					},
					"B": &types.AttributeValueMemberBOOL{Value: false},
				},
			},
		},
		{
			name: "Interfaces/Map/SkipNull",
			goValue: map[string]MockDynamoMarshaler{
				"x": {}, //will be encoded to go nil and will b skipped
			},
			dynamoValue: &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
		},
		{
			name: "Interfaces/NestedAny/Error",
			goValue: struct{ X any }{
				X: map[string]any{
					"ErrorKey": MockDynamoMarshaler{Value: nil, Err: fmt.Errorf("mock-error")},
				},
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[map[string]any](),
					Err: serializer.NewMarshalError(serializer.SerializerErrorData{
						GoType: reflect.TypeFor[MockDynamoMarshaler](),
						Err:    fmt.Errorf("mock-error"),
					}),
				},
			),
		},
		{
			name: "Interfaces/SkipNull",
			goValue: struct{ X any }{
				X: []any{
					MockDynamoMarshaler{Value: &types.AttributeValueMemberBOOL{Value: true}},
					MockDynamoMarshaler{Value: &types.AttributeValueMemberNULL{Value: true}},
					MockDynamoMarshaler{Value: nil, Err: nil}, //will be skipped
				},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberBOOL{Value: true},
							&types.AttributeValueMemberNULL{Value: true},
						},
					},
				},
			},
		},
		{
			name:    "Interfaces/Any/Maps/Nil",
			goValue: struct{ X any }{map[string]any(nil)},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
				},
			},
		},
		{
			name: "Interfaces/Slice",
			goValue: []any{
				false,
				true,
				"hello",
				[]byte("world"),
				int32(-32),
				namedInt64(-64),
				uint32(+32),
				namedUint64(+64),
				float32(32.32),
				namedFloat64(64.64),
			},
			dynamoValue: &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberBOOL{Value: false},
					&types.AttributeValueMemberBOOL{Value: true},
					&types.AttributeValueMemberS{Value: "hello"},
					&types.AttributeValueMemberB{Value: []byte("world")},
					&types.AttributeValueMemberN{Value: "-32"},
					&types.AttributeValueMemberN{Value: "-64"},
					&types.AttributeValueMemberN{Value: "32"},
					&types.AttributeValueMemberN{Value: "64"},
					&types.AttributeValueMemberN{Value: "32.32"},
					&types.AttributeValueMemberN{Value: "64.64"},
				},
			},
		},
		{
			name:    "Interfaces/Any/Named",
			goValue: struct{ X namedAny }{[]namedAny{nil, false, "", 0.0, map[string]namedAny{}, []namedAny{}, [8]byte{}}},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{
							&types.AttributeValueMemberNULL{Value: true},
							&types.AttributeValueMemberBOOL{Value: false},
							&types.AttributeValueMemberS{Value: ""},
							&types.AttributeValueMemberN{Value: "0"},
							&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
							&types.AttributeValueMemberL{Value: []types.AttributeValue{}},
							&types.AttributeValueMemberB{Value: []byte{0, 0, 0, 0, 0, 0, 0, 0}},
						},
					},
				},
			},
		},
		{
			name:    "Interfaces/Any/Slices/Empty",
			goValue: struct{ X any }{[]any{}},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberL{
						Value: []types.AttributeValue{},
					},
				},
			},
		},
		{
			name: "Interfaces/Any/List/Error",
			goValue: struct{ X any }{
				X: []any{
					MockDynamoMarshaler{Value: nil, Err: fmt.Errorf("mock-error")}, //
				},
			},
			expectedError: serializer.NewMarshalError(
				serializer.SerializerErrorData{
					Kind:   serializer.DynamoKindMap,
					GoType: reflect.TypeFor[[]any](),
					Err: serializer.NewMarshalError(serializer.SerializerErrorData{
						GoType: reflect.TypeFor[MockDynamoMarshaler](),
						Err:    fmt.Errorf("mock-error"),
					},
					),
				},
			),
		},
		{
			name: "Interfaces/Any/MapSkipNul",
			goValue: struct{ X any }{
				X: map[string]any{
					"Skipped": MockDynamoMarshaler{Value: nil, Err: nil}, //Will be skipped
					"Null":    nil,
					"Str":     "a",
				},
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"X": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"Null": &types.AttributeValueMemberNULL{Value: true},
							"Str":  &types.AttributeValueMemberS{Value: "a"},
						},
					},
				},
			},
		},
		{
			name: "Interfaces/Slice/MockDynamoMarshaler/SkipNull",
			goValue: []MockDynamoMarshaler{
				{Value: nil, Err: nil},
			},
			dynamoValue: &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
		},
		{
			name: "Interfaces/MockDynamoMarshaler",
			goValue: MockDynamoMarshaler{
				Value: &types.AttributeValueMemberBOOL{Value: true},
			},
			dynamoValue: &types.AttributeValueMemberBOOL{Value: true},
		},
		{
			name: "Interfaces/MockDynamoMarshalerPtr",
			goValue: MockDynamoMarshalerPtr{
				Value: &types.AttributeValueMemberBOOL{Value: true},
			},
			dynamoValue: &types.AttributeValueMemberBOOL{Value: true},
		},
		{
			name: "Interfaces/TextMarshalerMockBytes",
			goValue: TextMarshalerMockBytes{
				Value: []byte("Abc"),
			},
			dynamoValue: &types.AttributeValueMemberS{Value: "Abc"},
		},
		{
			name: "Interfaces/TextMarshalerMockBytes/Null",
			goValue: TextMarshalerMockBytes{
				Value: []byte(nil),
			},
			dynamoValue: &types.AttributeValueMemberNULL{Value: true},
		},
		{
			name: "Interfaces/TextMarshalerMock",
			goValue: TextMarshalerMock{
				Value: "123",
			},
			dynamoValue: &types.AttributeValueMemberS{Value: "123"},
		},
		{
			name: "Interfaces/TextMarshalerMockPtr",
			goValue: TextMarshalerMockPtr{
				Value: "123",
			},
			dynamoValue: &types.AttributeValueMemberS{Value: "123"},
		},
		{
			name: "Interfaces/Ptr/textMarshalerMockPtr",
			goValue: &TextMarshalerMockPtr{
				Value: "123",
			},
			dynamoValue: &types.AttributeValueMemberS{Value: "123"},
		},
		{
			name: "Interfaces/BinaryMarshalerMock",
			goValue: binaryMarshalerMock{
				Value: "123",
			},
			dynamoValue: &types.AttributeValueMemberB{Value: []byte("123")},
		},
		{
			name: "Interfaces/BinaryMarshalerMockPtr",
			goValue: binaryMarshalerMockPtr{
				Value: "123",
			},
			dynamoValue: &types.AttributeValueMemberB{Value: []byte("123")},
		},
		{
			name: "Set/InvalidType/Nil+Omitempty",
			goValue: struct {
				IpSet []net.IPAddr `dynamodbav:",set,omitempty"`
			}{},
			expectedError: nil,
			dynamoValue:   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
		},
		{
			name: "Set/InvalidType",
			goValue: struct {
				IpSet []net.IPAddr `dynamodbav:",set"`
			}{
				[]net.IPAddr{},
			},
			expectedError: serializer.NewMarshalError(serializer.SerializerErrorData{
				GoType: reflect.TypeFor[[]net.IPAddr](),
				Err:    fmt.Errorf("cant marshal item as set:%w", errors.ErrUnsupported),
			}),
		},
		{
			name: "Set/InvalidType/Nil",
			goValue: struct {
				IpSet []net.IPAddr `dynamodbav:",set"`
			}{
				nil,
			},
			expectedError: serializer.NewMarshalError(serializer.SerializerErrorData{
				GoType: reflect.TypeFor[[]net.IPAddr](),
				Err:    fmt.Errorf("cant marshal item as set:%w", errors.ErrUnsupported),
			}),
		},
		{
			name:        "StdlibNet/NetIPV4/TextMarshaler", //ipv4 implements encoding.TextMarshaler
			goValue:     net.IPv4(192, 168, 0, 100),
			dynamoValue: &types.AttributeValueMemberS{Value: "192.168.0.100"},
		},
		{
			name: "StdlibNet/NetIP/TextMarshaler",
			goValue: struct {
				Addr     netip.Addr
				AddrPort netip.AddrPort
				Prefix   netip.Prefix
			}{
				Addr:     netip.AddrFrom4([4]byte{1, 2, 3, 4}),
				AddrPort: netip.AddrPortFrom(netip.AddrFrom4([4]byte{1, 2, 3, 4}), 1234),
				Prefix:   netip.PrefixFrom(netip.AddrFrom4([4]byte{1, 2, 3, 4}), 24),
			},
			dynamoValue: &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"Addr":     &types.AttributeValueMemberS{Value: "1.2.3.4"},
					"AddrPort": &types.AttributeValueMemberS{Value: "1.2.3.4:1234"},
					"Prefix":   &types.AttributeValueMemberS{Value: "1.2.3.4/24"},
				},
			},
		},
	}

	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			r := require.New(t)
			av, err := serializer.Marshal(testcase.goValue, testcase.opts...)
			if testcase.expectedError == nil {
				r.NoError(err)
				r.Equal(testcase.dynamoValue, av)
			} else {
				r.Error(err)
				r.EqualError(err, testcase.expectedError.Error())
			}
		},
		)
	}
}

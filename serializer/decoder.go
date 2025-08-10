package serializer

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type decoderState struct {
}

type DecoderOption interface {
	applyDecoderOption(*DecoderConfig)
}

var _ = decoderOptionAdapter(nil)

type decoderOptionAdapter func(*DecoderConfig)

// apply implements EncoderOption.
func (o decoderOptionAdapter) applyDecoderOption(cfg *DecoderConfig) {
	o(cfg)
}

type ArrayUnmarshalOverflowOption uint8

// return true if decoder is allowed to ignore items when unmarshaling array incase array length is smaller then data slice length
func (overFlowOption ArrayUnmarshalOverflowOption) IsOverflowAllowed() bool {
	return overFlowOption == AllowUnmarshalArrayFromAnyLen || overFlowOption == CheckUnderflowArrayOnUnmarshaling
}

// return true if decoder is allowed to zero items when unmarshaling array incase array length is larger then data slice length
func (overFlowOption ArrayUnmarshalOverflowOption) IsUnderflowAllowed() bool {
	return overFlowOption == AllowUnmarshalArrayFromAnyLen || overFlowOption == CheckOverflowUnmarshalArrayOnUnmarshaling
}

const (
	//fail on attempt to unmarshal into go array if the data length is not exact match to the array length
	CheckOverFlowAndUnderflowArrayOnUnmarshal ArrayUnmarshalOverflowOption = iota //0b0
	//allow unmarshal into go array if the data length is longer then the array length - any data that is beyond the array length will be discarded
	CheckUnderflowArrayOnUnmarshaling ArrayUnmarshalOverflowOption = iota //0b01
	//allow unmarshal into go array if the data length is shorter then the array length - rest of the array will contain the zero value for the un-marshaled type
	CheckOverflowUnmarshalArrayOnUnmarshaling ArrayUnmarshalOverflowOption = iota //0b10
	//will allow to attempt unmarshaling into go array data of any length the behavior will be like other options described
	AllowUnmarshalArrayFromAnyLen ArrayUnmarshalOverflowOption = iota //0b11
)

type DecoderConfig struct {
	unmarshalArrayBoundsCheck ArrayUnmarshalOverflowOption
	unmarshalersRegistry      *customDeserializerRegistry
	tag                       string
}

func WithArrayBoundCheckMode(m ArrayUnmarshalOverflowOption) DecoderOption {
	return decoderOptionAdapter(
		func(dc *DecoderConfig) {
			dc.unmarshalArrayBoundsCheck = m
		},
	)
}

func WithUnmarshalers(m customUnMarshaler, more ...customUnMarshaler) DecoderOption {
	return decoderOptionAdapter(
		func(dc *DecoderConfig) {
			if dc.unmarshalersRegistry == nil {
				dc.unmarshalersRegistry = &customDeserializerRegistry{}
			}
			dc.unmarshalersRegistry.addUnmarshaler(m)
			for _, unmarshaler := range more {
				dc.unmarshalersRegistry.addUnmarshaler(unmarshaler)
			}
		},
	)
}

func NewDecoderConfig(opts ...DecoderOption) DecoderConfig {
	cfg := DecoderConfig{
		tag: "dynamodbav",
	}
	for _, o := range opts {
		o.applyDecoderOption(&cfg)
	}
	return cfg
}

type Decoder struct {
	config DecoderConfig
}

func NewDecoder(opts ...DecoderOption) *Decoder {
	cfg := NewDecoderConfig(opts...)
	return &Decoder{
		config: cfg,
	}
}

var ErrInvalidUnmarshalInputType = fmt.Errorf("unmarshal input have to be non nil pointer:%w", errors.ErrUnsupported)

func (d *Decoder) Unmarshal(av types.AttributeValue, out any) (err error) {
	v := reflect.ValueOf(out)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return newUnMarshalError(
			SerializerErrorData{
				Kind: dynamodbAvToKind(av),
				Err:  ErrInvalidUnmarshalInputType},
		)
	}
	val := addressableValue{v.Elem()}
	t := val.Type()
	err = lookupSerializer(t).unmarshal(d, av, val, decoderState{})
	return err
}

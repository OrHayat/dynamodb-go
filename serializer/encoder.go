package serializer

import (
	"reflect"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Encoder struct {
	config EncoderConfig
}

func NewEncoder(opts ...EncoderOption) *Encoder {
	cfg := NewEncoderConfig(opts...)
	return &Encoder{
		config: cfg,
	}
}

type EncoderConfig struct {
	tag                  string
	marshalersRegistry   *customEncoderRegistry
	omitZeroStructFields bool
}

type EncoderOption interface {
	applyEncoderOpt(*EncoderConfig)
}

var _ EncoderOption = encoderOptionAdapter(nil)

type encoderOptionAdapter func(*EncoderConfig)

// apply implements EncoderOption.
func (o encoderOptionAdapter) applyEncoderOpt(cfg *EncoderConfig) {
	o(cfg)
}

func NewEncoderConfig(opts ...EncoderOption) EncoderConfig {
	cfg := EncoderConfig{
		tag:                  "dynamodbav",
		omitZeroStructFields: false,
	}
	for _, o := range opts {
		o.applyEncoderOpt(&cfg)
	}
	return cfg
}

var _ EncoderOption = tagOption{}
var _ DecoderOption = tagOption{}

type tagOption struct {
	tag string
}

// applyDecoderOption implements DecoderOption.
func (t tagOption) applyDecoderOption(cfg *DecoderConfig) {
	cfg.tag = t.tag
}

func WithTag(tag string) tagOption {
	return tagOption{
		tag: tag,
	}
}

func WithMarshalers(marshaler customMarshaler, more ...customMarshaler) EncoderOption {
	return encoderOptionAdapter(
		func(ec *EncoderConfig) {
			if ec.marshalersRegistry == nil {
				ec.marshalersRegistry = &customEncoderRegistry{}
			}
			ec.marshalersRegistry.addMarshaler(marshaler)
			for _, m := range more {
				ec.marshalersRegistry.addMarshaler(m)
			}
		},
	)
}

// applyEncoderOpt implements EncoderOption.
func (t tagOption) applyEncoderOpt(cfg *EncoderConfig) {
	cfg.tag = t.tag
}

func OmitZeroStructFields() encoderOptionAdapter {
	return encoderOptionAdapter(func(ec *EncoderConfig) {
		ec.omitZeroStructFields = true
	})
}

type encoderState struct {
	AsSet bool
}

func Marshal(in any, opts ...EncoderOption) (av types.AttributeValue, err error) {
	return NewEncoder(opts...).Marshal(in)
}

func (e *Encoder) Marshal(in any) (av types.AttributeValue, err error) {

	v := reflect.ValueOf(in)
	if !v.IsValid() || (v.Kind() == reflect.Pointer && v.IsNil()) {
		return &types.AttributeValueMemberNULL{Value: true}, nil
	}
	//if the value is pointer you can dereference it and the dereferenced value will be addressable
	//otherwise create new empty value with reflection that will be addressable
	if v.Kind() != reflect.Pointer {
		v2 := reflect.New(v.Type())
		v2.Elem().Set(v)
		v = v2
	}
	va := addressableValue{v.Elem()} // dereferenced pointer is always addressable
	t := va.Type()
	s := lookupSerializer(t) //s is always valid - it either an existing serializer or new one - every type can create serializer and store it in the registry
	flags := encoderState{}
	return s.marshal(e, va, flags)
}

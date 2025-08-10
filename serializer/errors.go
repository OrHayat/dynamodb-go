package serializer

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const errorPrefix = "dynamo: "

type DynamoKind int8

const (
	DynamoKindUnknown   DynamoKind = iota
	DynamoKindNull      DynamoKind = iota
	DynamoKindString    DynamoKind = iota
	DynamoKindNumber    DynamoKind = iota
	DynamoKindBool      DynamoKind = iota
	DynamoKindBinary    DynamoKind = iota
	DynamoKindList      DynamoKind = iota
	DynamoKindMap       DynamoKind = iota
	DynamoKindStringSet DynamoKind = iota
	DynamoKindNumberSet DynamoKind = iota
	DynamoKindBinarySet DynamoKind = iota
)

type SerializerErrorData struct {
	Path   string
	Kind   DynamoKind   //optional
	GoType reflect.Type //may be nill
	Err    error        //may be nill if there is no underlying error
}

func (e SerializerErrorData) Unwrap(err error) error {
	return e.Err
}

func newSerializerError(data SerializerErrorData) *SerializerError {
	return &SerializerError{
		action:              "",
		SerializerErrorData: data,
	}
}

func newMarshalError(data SerializerErrorData) *SerializerError {
	return &SerializerError{
		action:              "marshal",
		SerializerErrorData: data,
	}
}

func newUnMarshalError(data SerializerErrorData) *SerializerError {
	return &SerializerError{
		action:              "unmarshal",
		SerializerErrorData: data,
	}
}

func NewUnSupportedTypeError(av types.AttributeValue, t reflect.Type) error {
	return newUnMarshalError(
		SerializerErrorData{
			Err:    fmt.Errorf("unsupported AttributeValue type, %T", av),
			Kind:   dynamodbAvToKind(av),
			GoType: t,
		},
	)
}

type SerializerError struct {
	action string //optional : only marshal and unmarshal actions are valid ,any other value will be ignored during formatting of the message
	SerializerErrorData
}

func (e SerializerError) Error() string {
	var sb strings.Builder
	sb.WriteString(errorPrefix)
	sb.WriteString("cannot")
	// Format action.
	var preposition string
	switch e.action {
	case "marshal":
		sb.WriteString(" marshal")
		preposition = " from"
	case "unmarshal":
		sb.WriteString(" unmarshal")
		preposition = " into"
	default:
		sb.WriteString(" handle")
		preposition = " with"
	}

	//Format kind.
	var omitPreposition = false
	switch e.Kind {
	case DynamoKindNull:
		sb.WriteString(" Dynamodb null")
	case DynamoKindString:
		sb.WriteString(" Dynamodb string")
	case DynamoKindNumber:
		sb.WriteString(" Dynamodb number")
	case DynamoKindBool:
		sb.WriteString(" Dynamodb bool")
	case DynamoKindBinary:
		sb.WriteString(" Dynamodb binary blob")
	case DynamoKindList:
		sb.WriteString(" Dynamodb list")
	case DynamoKindMap:
		sb.WriteString(" Dynamodb map")
	case DynamoKindStringSet:
		sb.WriteString(" Dynamodb string set")
	case DynamoKindNumberSet:
		sb.WriteString(" Dynamodb number set")
	case DynamoKindBinarySet:
		sb.WriteString(" Dynamodb binary blobs set")
	default:
		omitPreposition = true
	}

	// Format Go type.
	if e.GoType != nil {
		if !omitPreposition {
			sb.WriteString(preposition)
		}
		sb.WriteString(" Go value of type ")
		sb.WriteString(e.GoType.String())
	}

	if e.Path != "" {
		sb.WriteString(" within attribute value at " + e.Path)
	}

	if e.Err != nil {
		sb.WriteString(": ")
		sb.WriteString(e.Err.Error())
	}

	return sb.String()
}

func dynamodbAvToKind(av types.AttributeValue) DynamoKind {
	switch av.(type) {
	case *types.AttributeValueMemberNULL:
		return DynamoKindNull
	case *types.AttributeValueMemberS:
		return DynamoKindString
	case *types.AttributeValueMemberN:
		return DynamoKindNumber
	case *types.AttributeValueMemberBOOL:
		return DynamoKindBool
	case *types.AttributeValueMemberB:
		return DynamoKindBinary
	case *types.AttributeValueMemberL:
		return DynamoKindList
	case *types.AttributeValueMemberM:
		return DynamoKindMap
	case *types.AttributeValueMemberSS:
		return DynamoKindStringSet
	case *types.AttributeValueMemberNS:
		return DynamoKindNumberSet
	case *types.AttributeValueMemberBS:
		return DynamoKindBinarySet
	default:
		return DynamoKindUnknown
	}
}

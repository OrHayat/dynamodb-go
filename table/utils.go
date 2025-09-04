package table

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// attributevalue.NewEncoder that encode time.Time as rfc339 instead of rfc339 nanos
func NewEncoder() *attributevalue.Encoder {
	return attributevalue.NewEncoder(func(eo *attributevalue.EncoderOptions) {
		eo.EncodeTime = encodeTimeRfc339
	})
}

func Marshal(in any) (types.AttributeValue, error) {
	return NewEncoder().Encode(in)
}

// same as attributevalue.MarshalMap but it encode time.Time to rfc339 instead of rfc339 nanos
func MarshalMap(in any) (map[string]types.AttributeValue, error) {
	return attributevalue.MarshalMapWithOptions(in,
		func(eo *attributevalue.EncoderOptions) { eo.EncodeTime = encodeTimeRfc339 })
}

func encodeTimeRfc339(t time.Time) (types.AttributeValue, error) {
	return &types.AttributeValueMemberS{Value: t.Format(time.RFC3339)}, nil
}

type FuncOption[T any] func(*T)

func ApplyOptions[OptionsType ~[]OptsType, OptsType ~func(*T), T any](item *T, optsFn OptionsType) {
	for _, fn := range optsFn {
		fn(item)
	}
}

type DynamoDBFuncOpts = func(*dynamodb.Options)

func addAttributesExistsToQueryBuilder(builder expression.ConditionBuilder, attributes map[string]types.AttributeValue) expression.ConditionBuilder {
	for keyName := range attributes {
		cond := expression.Name(keyName).AttributeExists()
		if builder.IsSet() {
			builder = builder.And(cond)
		} else {
			builder = cond
		}
	}
	return builder
}

func addAttributesDontExistsToQueryBuilder(builder expression.ConditionBuilder, attributes map[string]types.AttributeValue) expression.ConditionBuilder {
	for keyName := range attributes {
		cond := expression.Name(keyName).AttributeNotExists()
		if builder.IsSet() {
			builder = builder.And(cond)
		} else {
			builder = cond
		}
	}
	return builder
}

func Unmarshal[T any](av types.AttributeValue) (out T, err error) {
	err = attributevalue.Unmarshal(av, &out)
	if err != nil {
		return out, err
	}
	return out, nil
}

func UnmarshalMap[T any](av map[string]types.AttributeValue) (out T, err error) {
	err = attributevalue.UnmarshalMap(av, &out)
	if err != nil {
		return out, err
	}
	return out, nil
}

func UnmarshalListOfMaps[T any](attributeValues []map[string]types.AttributeValue) (items []T, err error) {
	err = attributevalue.UnmarshalListOfMaps(attributeValues, &items)
	if err != nil {
		return items, err
	}
	return items, nil
}

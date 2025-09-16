package table

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/orhayat/dynamodb-go/serializer"
)

type GetItemClient interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	GetDecoder() *serializer.Decoder //mil imply to use default decoder
}

type GetItemConfig struct {
	Consistency bool
	Decoder     *serializer.Decoder
}

type GetItemOptions interface {
	applyGetItemOption(*GetItemConfig)
}

func prepareGetRequest(
	cfg GetItemConfig,
	tableName string,
	key map[string]types.AttributeValue,
) *dynamodb.GetItemInput {
	return &dynamodb.GetItemInput{
		TableName:                &tableName,
		Key:                      key,
		ProjectionExpression:     nil, //TODO:add way to generate it
		ExpressionAttributeNames: nil, //needed for projection expression incase of unsupported word in the expression useful to not fetch whole record of table across the wire if only part of it needed
		ConsistentRead:           aws.Bool(cfg.Consistency),
		ReturnConsumedCapacity:   types.ReturnConsumedCapacityNone, //safe default- usefull for metrics but this package dont help to export metrics
	}
}

func GetItem(
	ctx context.Context,
	client GetItemClient,
	table *TableDefinition,
	key Key,
	out any,
	opts ...GetItemOptions,
) (err error) {
	cfg := GetItemConfig{
		Consistency: true,
		Decoder:     client.GetDecoder(),
	}
	for _, o := range opts {
		o.applyGetItemOption(&cfg)
	}

	encodedKey, err := table.getKey(key)
	if err != nil {
		return &OperationError{
			operation:   "get item key encoding",
			table:       table,
			pk:          key.PK,
			sk:          key.SK,
			internalErr: err,
		}
	}

	request := prepareGetRequest(cfg, table.Name, encodedKey)
	res, err := client.GetItem(ctx, request)
	if err != nil {
		return &OperationError{
			operation:   "get item",
			table:       table,
			pk:          key.PK,
			sk:          key.SK,
			internalErr: err,
		}
	}

	if len(res.Item) == 0 {
		return &OperationError{
			operation:   "get item",
			table:       table,
			pk:          key.PK,
			sk:          key.SK,
			internalErr: ErrItemNotFound,
		}
	}

	err = serializer.UnmarshalMap(cfg.Decoder, res.Item, out)
	if err != nil {
		return &OperationError{
			operation:   "get item unmarshal",
			table:       table,
			pk:          key.PK,
			sk:          key.SK,
			internalErr: err,
		}
	}
	return nil
}

func GetItemOf[T any](
	ctx context.Context,
	client GetItemClient,
	table *TableDefinition,
	key Key,
	opts ...GetItemOptions,
) (out T, err error) {
	err = GetItem(ctx, client, table, key, &out, opts...)
	return
}

func GetAsJSON(
	ctx context.Context,
	client GetItemClient,
	table *TableDefinition,
	key Key,
	opts ...GetItemOptions,
) (out map[string]any, err error) {
	var res any
	err = GetItem(ctx, client, table, key, &res, opts...)
	if err != nil {
		return nil, err
	}
	return res.(map[string]any), nil
}

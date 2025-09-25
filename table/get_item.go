package table

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/orhayat/dynamodb-go/serializer"
)

type GetItemClient interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	GetDecoder() *serializer.Decoder //mil imply to use default decoder
}

type ProjectionBuilder struct {
	projectionBuilder expression.ProjectionBuilder
	projectionEnabled bool //marker to know if projection was set by user or not
}

type GetItemConfig struct {
	Consistency bool
	Decoder     *serializer.Decoder
	ProjectionBuilder
	// Projection        expression.ProjectionBuilder
	// projectionEnabled bool //marker to know if projection was set by user or not
}

type GetItemOptions interface {
	applyGetItemOption(*GetItemConfig)
}

func prepareGetRequest(
	cfg GetItemConfig,
	tableName string,
	key map[string]types.AttributeValue,
) *dynamodb.GetItemInput {

	var projection *string
	var names map[string]string
	if cfg.projectionEnabled {
		b := expression.NewBuilder().WithProjection(cfg.projectionBuilder)
		expr, err := b.Build()
		if err == nil {
			projection = expr.Projection()
			names = expr.Names()
		}
	}

	return &dynamodb.GetItemInput{
		TableName:                &tableName,
		Key:                      key,
		ProjectionExpression:     projection,
		ExpressionAttributeNames: names,
		ConsistentRead:           aws.Bool(cfg.Consistency),
		ReturnConsumedCapacity:   "", //TODO: add way to return consumed capacity
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
		Decoder:     nil,
	}
	for _, o := range opts {
		o.applyGetItemOption(&cfg)
	}
	if cfg.Decoder == nil {
		cfg.Decoder = client.GetDecoder()
	}
	if cfg.Decoder == nil {
		cfg.Decoder = s_decoder
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

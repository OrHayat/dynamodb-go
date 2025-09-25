package table

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/serializer"
)

type BatchGetItemClient interface {
	BatchGetItem(ctx context.Context, params *dynamodb.BatchGetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.BatchGetItemOutput, error)
	GetDecoder() *serializer.Decoder
}

type BatchGetItemOptions interface {
	applyBatchGetItemOption(*BatchGetItemConfig)
}

type BatchGetItemConfig struct {
	ConsistentRead bool
	Decoder        *serializer.Decoder
	ProjectionBuilder
}

func prepareBatchGetItemRequestSingleTable(
	cfg *BatchGetItemConfig,
	table *TableDefinition,
	keys []Key,

) (request *dynamodb.BatchGetItemInput, err error) {
	var keysToGet []map[string]types.AttributeValue
	for _, key := range keys {
		encodedKey, err := table.getKey(key)
		if err != nil {
			return nil, &OperationError{
				operation:   "batch get item",
				table:       table,
				internalErr: err,
			}
		}
		keysToGet = append(keysToGet, encodedKey)
	}
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
	tableRequest := types.KeysAndAttributes{
		ConsistentRead:           aws.Bool(cfg.ConsistentRead),
		Keys:                     keysToGet,
		ProjectionExpression:     projection,
		ExpressionAttributeNames: names,
	}

	requestedItems := map[string]types.KeysAndAttributes{
		table.Name: tableRequest,
	}

	return &dynamodb.BatchGetItemInput{
		RequestItems:           requestedItems,
		ReturnConsumedCapacity: "",
	}, nil
}

type BatchGetItemInput struct {
	Table *TableDefinition
	Keys  []Key
}

func BatchGetItemSingleTable(
	ctx context.Context,
	client BatchGetItemClient,
	table *TableDefinition,
	keys []Key,
	out any,
	options ...BatchGetItemOptions,
) error {
	cfg := BatchGetItemConfig{
		ConsistentRead: false,
		Decoder:        nil,
	}
	for _, opt := range options {
		opt.applyBatchGetItemOption(&cfg)
	}
	if cfg.Decoder == nil {
		cfg.Decoder = client.GetDecoder()
	}
	if cfg.Decoder == nil {
		cfg.Decoder = s_decoder
	}
	request, err := prepareBatchGetItemRequestSingleTable(&cfg, table, keys)
	if err != nil {
		return err
	}

	attempt := 0
	res := []map[string]types.AttributeValue{}
	for {
		response, err := client.BatchGetItem(ctx, request)
		if err != nil {
			return err
		}
		res = append(res, response.Responses[table.Name]...)
		request.RequestItems = response.UnprocessedKeys
		if len(response.UnprocessedKeys) == 0 {
			break
		}
		delay := backoffDelay(attempt, 15, time.Millisecond*300)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// continue and do the next attempt
		}
		attempt++
	}
	err = serializer.UnmarshalListOfMaps(cfg.Decoder, res, out)
	if err != nil {
		return &OperationError{
			operation:   "batch get item unmarshal",
			table:       table,
			internalErr: err,
		}
	}
	return nil
}

func BatchGetItemsFromSingleTable[T any](
	ctx context.Context,
	client BatchGetItemClient,
	table *TableDefinition,
	keys []Key,
	options ...BatchGetItemOptions,
) ([]T, error) {
	var out []T
	err := BatchGetItemSingleTable(ctx, client, table, keys, &out, options...)
	return out, err
}

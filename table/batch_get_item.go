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
}

func prepareBatchGetItemRequestSingleTable(
	table *TableDefinition,
	input BatchGetSingleTableInput,

) (request *dynamodb.BatchGetItemInput, err error) {
	var keysToGet []map[string]types.AttributeValue
	for _, key := range input.Keys {
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
	expr, err := prepareGetExpression(input.ProjectionExpression)
	if err != nil {
		return nil, err
	}

	tableRequest := types.KeysAndAttributes{
		ConsistentRead:           aws.Bool(input.ConsistentRead.Bool()),
		Keys:                     keysToGet,
		ProjectionExpression:     expr.Projection(),
		ExpressionAttributeNames: expr.Names(),
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

type BatchGetSingleTableInput struct {
	Keys                 []Key
	ConsistentRead       aws.Ternary
	ProjectionExpression *expression.ProjectionBuilder
}

func BatchGetItemSingleTable(
	ctx context.Context,
	client BatchGetItemClient,
	table *TableDefinition,
	input BatchGetSingleTableInput,
	out any,
	options ...BatchGetItemOptions,
) error {
	decoder := client.GetDecoder()
	if decoder == nil {
		decoder = s_decoder
	}
	request, err := prepareBatchGetItemRequestSingleTable(table, input)
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
	err = serializer.UnmarshalListOfMaps(decoder, res, out)
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
	input BatchGetSingleTableInput,
	options ...BatchGetItemOptions,
) ([]T, error) {
	var out []T
	err := BatchGetItemSingleTable(ctx, client, table, input, &out, options...)
	return out, err
}

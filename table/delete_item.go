package table

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DeleteItemClient interface {
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
}

type deleteItemNotExistsCheckMode int

const (
	conditionaExpressionForItemNotExists deleteItemNotExistsCheckMode = iota
)

func prepareDeleteItemRequest(
	table *TableDefinition,
	encodedKey map[string]types.AttributeValue,
) (request *dynamodb.DeleteItemInput, mode deleteItemNotExistsCheckMode, err error) {
	mode = conditionaExpressionForItemNotExists //for now thats the only supported mode....
	b := expression.NewBuilder()
	condition := expression.AttributeExists(expression.Name(table.PrimaryKey.Name))
	if table.RangeKey.Name != "" {
		condition = condition.And(expression.AttributeExists(expression.Name(table.RangeKey.Name)))
	}
	b = b.WithCondition(condition)
	exp, err := b.Build()
	if err != nil {
		return nil, mode, fmt.Errorf("failed to build expression: %w", err)
	}
	return &dynamodb.DeleteItemInput{
		TableName:                   &table.Name,
		Key:                         encodedKey,
		ConditionExpression:         exp.Condition(),
		ExpressionAttributeNames:    exp.Names(),
		ExpressionAttributeValues:   exp.Values(),
		ReturnConsumedCapacity:      "", //safe default- usefull for metrics but this package dont help to export metricss
		ReturnItemCollectionMetrics: "", //can be used to notify/log collections that are close to getting
		ReturnValues:                "", //
	}, mode, nil
}

func DeleteItem(
	ctx context.Context,
	client DeleteItemClient,
	table *TableDefinition,
	pk any,
	sk any,
) (err error) {
	encodedKey, err := table.getKey(pk, sk)
	if err != nil {
		return &OperationError{
			operation:   "delete item key encoding",
			table:       table,
			pk:          pk,
			sk:          sk,
			internalErr: err,
		}
	}
	request, mode, err := prepareDeleteItemRequest(table, encodedKey)
	if err != nil {
		return &OperationError{
			operation:   "delete item request prepration",
			table:       table,
			pk:          pk,
			sk:          sk,
			internalErr: err,
		}
	}
	_, err = client.DeleteItem(ctx, request)
	if err != nil {
		if mode == conditionaExpressionForItemNotExists {
			if _, ok := ErrorAs[*types.ConditionalCheckFailedException](err); ok {
				return &OperationError{
					operation:   "delete item",
					table:       table,
					pk:          pk,
					sk:          sk,
					internalErr: ErrItemNotFound,
				}
			}
		}
		return &OperationError{
			operation:   "delete item",
			table:       table,
			pk:          pk,
			sk:          sk,
			internalErr: err,
		}
	}
	return nil
}

func ErrorAs[T error](err error) (res T, ok bool) {
	ok = errors.As(err, &res)
	return
}

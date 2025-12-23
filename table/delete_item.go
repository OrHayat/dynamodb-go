package table

import (
	"context"
	"errors"

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

func prepareDeleteItemExpression(table *TableDefinition, conditionalCheck expression.ConditionBuilder) (expr expression.Expression, err error) {
	b := expression.NewBuilder()
	if !conditionalCheck.IsSet() {
		conditionalCheck = ensureKeyExists(table)
	} else {
		conditionalCheck = ensureKeyExists(table).And(conditionalCheck)
	}
	b = b.WithCondition(conditionalCheck)
	return b.Build()
}

func prepareDeleteItemRequest(
	table *TableDefinition,
	encodedKey map[string]types.AttributeValue,
	conditionalCheck expression.ConditionBuilder,
) (request *dynamodb.DeleteItemInput, mode deleteItemNotExistsCheckMode, err error) {
	mode = conditionaExpressionForItemNotExists //for now thats the only supported mode....
	expr, err := prepareDeleteItemExpression(table, conditionalCheck)
	if err != nil {
		return nil, mode, err
	}
	return &dynamodb.DeleteItemInput{
		TableName:                           &table.Name,
		Key:                                 encodedKey,
		ConditionExpression:                 expr.Condition(),
		ExpressionAttributeNames:            expr.Names(),
		ExpressionAttributeValues:           expr.Values(),
		ReturnConsumedCapacity:              "", //safe default- usefull for metrics but this package dont help to export metricss
		ReturnItemCollectionMetrics:         "", //can be used to notify/log collections that are close to getting
		ReturnValues:                        "", //
		ReturnValuesOnConditionCheckFailure: types.ReturnValuesOnConditionCheckFailureAllOld,
	}, mode, nil
}

type DeleteItemInput struct {
	//key of deleted item
	Key Key
	//additional conditions that can be used to limit the delete item operation
	ConditionalCheck expression.ConditionBuilder
}

func DeleteItem(
	ctx context.Context,
	client DeleteItemClient,
	table *TableDefinition,
	input DeleteItemInput,
) (err error) {
	key := input.Key
	encodedKey, err := table.getKey(key)
	if err != nil {
		return &OperationError{
			operation:   "delete item key encoding",
			table:       table,
			pk:          key.PK,
			sk:          key.SK,
			internalErr: err,
		}
	}
	request, mode, err := prepareDeleteItemRequest(table, encodedKey, input.ConditionalCheck)
	if err != nil {
		return &OperationError{
			operation:   "delete item request prepration",
			table:       table,
			pk:          key.PK,
			sk:          key.SK,
			internalErr: err,
		}
	}
	_, err = client.DeleteItem(ctx, request)
	if err != nil {
		if mode == conditionaExpressionForItemNotExists {
			if e, ok := ErrorAs[*types.ConditionalCheckFailedException](err); ok {
				if e.Item == nil {
					return &OperationError{
						operation:   "delete item",
						table:       table,
						pk:          key.PK,
						sk:          key.SK,
						internalErr: ErrItemNotFound,
					}
				}
			}
		}
		return &OperationError{
			operation:   "delete item",
			table:       table,
			pk:          key.PK,
			sk:          key.SK,
			internalErr: err,
		}
	}
	return nil
}

func ErrorAs[T error](err error) (res T, ok bool) {
	ok = errors.As(err, &res)
	return
}

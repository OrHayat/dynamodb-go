package table

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/serializer"
)

type TransactionWriteItemClient interface {
	TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
	GetEncoder() *serializer.Encoder
}

type TransactionWriteItemRequest struct {
	Table  *TableDefinition
	Checks []TransactionCheckRequest
}

type TransactionCheckRequest struct {
	Key       Key
	Condition expression.ConditionBuilder //required
}

func prepareTransactionCheckRequest(table *TableDefinition, check TransactionCheckRequest) (*types.ConditionCheck, error) {
	encodedKey, err := table.getKey(check.Key)
	if err != nil {
		return nil, err
	}
	if !check.Condition.IsSet() {
		return nil, fmt.Errorf("condition is not set")
	}
	b := expression.NewBuilder().WithCondition(check.Condition)
	expr, err := b.Build()
	if err != nil {
		return nil, err
	}

	return &types.ConditionCheck{
		Key:                                 encodedKey,
		TableName:                           aws.String(table.Name),
		ConditionExpression:                 expr.Condition(),
		ExpressionAttributeNames:            expr.Names(),
		ExpressionAttributeValues:           expr.Values(),
		ReturnValuesOnConditionCheckFailure: types.ReturnValuesOnConditionCheckFailureAllOld,
	}, nil
}

type TransactionPutRequest struct {
	Item                 any
	Condtion             expression.ConditionBuilder //optional
	AllowReplaceExisitng bool
}

func prepareTransactionPutRequest(table *TableDefinition, encoder *serializer.Encoder, request TransactionPutRequest) (*types.Put, error) {

	encodedItem, err := serializer.MarshalMap(encoder, request.Item)
	if err != nil {
		return nil, err
	}
	b := expression.NewBuilder()
	cond := request.Condtion
	if request.AllowReplaceExisitng {
		if cond.IsSet() {
			//if condition on the key exists -assume key exists
			// since replacement is allowed or that condition with keyNotExists condition
			cond = ensureKeyNotExists(table).Or(cond)
		}
	} else {
		//replacement is not allowed - ensure key doesnt exists
		cond = ensureKeyNotExists(table)
	}

	var expressionNames map[string]string
	var expressionValues map[string]types.AttributeValue
	var condition *string
	if cond.IsSet() {
		b = b.WithCondition(cond)
		expr, err := b.Build()
		if err != nil {
			return nil, err
		}
		expressionNames = expr.Names()
		expressionValues = expr.Values()
		condition = expr.Condition()
	}

	return &types.Put{
		Item:                                encodedItem,
		TableName:                           aws.String(table.Name),
		ConditionExpression:                 condition,
		ExpressionAttributeNames:            expressionNames,
		ExpressionAttributeValues:           expressionValues,
		ReturnValuesOnConditionCheckFailure: "",
	}, nil

}

type TransactionDeleteRequest struct {
	Key      Key
	Condtion expression.ConditionBuilder //optional - item assumed to exists if this is passed
	// AllowReplaceExisitng bool
}

// TODO: return deleteItemNotExistsCheckMode - and ensure main loop handle it with errors.As
func prepareTransactionDeleteRequest(
	table *TableDefinition,
	deleteRequest TransactionDeleteRequest,
) (request *types.Delete, err error) {
	encodedKey, err := table.getKey(deleteRequest.Key)
	if err != nil {
		return nil, err
	}

	var expressionNames map[string]string
	var expressionValues map[string]types.AttributeValue
	var condition *string
	if deleteRequest.Condtion.IsSet() {
		cond := ensureKeyExists(table).And(deleteRequest.Condtion)
		b := expression.NewBuilder().WithCondition(cond)
		expr, err := b.Build()
		if err != nil {
			return nil, err
		}
		condition = expr.Condition()
		expressionNames = expr.Names()
		expressionValues = expr.Values()
	}

	return &types.Delete{
		Key:                                 encodedKey,
		TableName:                           aws.String(table.Name),
		ConditionExpression:                 condition,
		ExpressionAttributeNames:            expressionNames,
		ExpressionAttributeValues:           expressionValues,
		ReturnValuesOnConditionCheckFailure: types.ReturnValuesOnConditionCheckFailureAllOld,
	}, nil
}

type TransactionUpdateRequest struct {
	Key
	ItemUpdate
}

func prepareTransactionUpdateRequest(
	table *TableDefinition,
	updateRequest TransactionUpdateRequest,
	encoder *serializer.Encoder,
) (*types.Update, error) {

	encodedKey, err := table.getKey(updateRequest.Key)
	if err != nil {
		return nil, err
	}

	expr, err := prepareUpdateExpression(
		table,
		encoder,
		updateRequest.ItemUpdate,
		updateRequest.PreventsOverwrite.Bool(),
		updateRequest.Upsert.Bool(),
		updateRequest.ConditionalCheck,
	)
	if err != nil {
		return nil, err
	}
	return &types.Update{
		Key:                                 encodedKey,
		TableName:                           aws.String(table.Name),
		UpdateExpression:                    expr.Update(),
		ConditionExpression:                 expr.Condition(),
		ExpressionAttributeNames:            expr.Names(),
		ExpressionAttributeValues:           expr.Values(),
		ReturnValuesOnConditionCheckFailure: types.ReturnValuesOnConditionCheckFailureAllOld,
	}, nil
}

// type WriteRequest struct {
// 	Table       *TableDefinition
// 	DeleteItems []DeleteRequest
// 	PutRequests []PutRequest
// }

type TransactionWriteItemOptions interface {
	applyTransactionWriteItemOption(cfg *TransactionWriteItemConfig)
}

type TransactionWriteItemConfig struct {
	Encoder *serializer.Encoder
}

func prepareWriteTransactionRequest() (*dynamodb.TransactWriteItemsInput, error) {
	request := &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				// Put: &types.Put{
				// 	Item:                      nil,
				// 	TableName:                 nil,
				// 	ConditionExpression:       nil,
				// 	ExpressionAttributeNames:  nil,
				// 	ExpressionAttributeValues: nil,
				// },
				Delete: &types.Delete{
					Key:                       nil,
					TableName:                 nil,
					ConditionExpression:       nil,
					ExpressionAttributeNames:  nil,
					ExpressionAttributeValues: nil,
				},
				Update: &types.Update{
					Key:                                 nil,
					TableName:                           nil,
					UpdateExpression:                    nil,
					ConditionExpression:                 nil,
					ExpressionAttributeNames:            nil,
					ExpressionAttributeValues:           nil,
					ReturnValuesOnConditionCheckFailure: "",
				},
				// ConditionCheck: &types.ConditionCheck{
				// 	Key:                                 nil,
				// 	TableName:                           nil,
				// 	ConditionExpression:                 nil,
				// 	ExpressionAttributeNames:            nil,
				// 	ExpressionAttributeValues:           nil,
				// 	ReturnValuesOnConditionCheckFailure: "",
				// },
			},
		},
	}
	return request, nil
}
func TransactionWriteItems(
	ctx context.Context,
	client TransactionWriteItemClient,
	writeRequests []TransactionWriteItemRequest,
	opts ...TransactionWriteItemOptions) (err error) {
	prepareWriteTransactionRequest()
	return nil
}

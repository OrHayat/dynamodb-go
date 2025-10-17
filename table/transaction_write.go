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
	Table   *TableDefinition
	Checks  []TransactionCheckRequest
	Updates []TransactionUpdateRequest
	Puts    []TransactionPutRequest
	Deletes []TransactionDeleteRequest
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

func prepareTransactionPutRequest(table *TableDefinition, request TransactionPutRequest, encoder *serializer.Encoder) (*types.Put, error) {

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
	expr, err := prepareDeleteItemExpression(table, deleteRequest.Condtion)
	if err != nil {
		return nil, err
	}
	return &types.Delete{
		Key:                                 encodedKey,
		TableName:                           aws.String(table.Name),
		ConditionExpression:                 expr.Condition(),
		ExpressionAttributeNames:            expr.Names(),
		ExpressionAttributeValues:           expr.Values(),
		ReturnValuesOnConditionCheckFailure: types.ReturnValuesOnConditionCheckFailureAllOld,
	}, nil
}

type TransactionUpdateRequest struct {
	Key
	UpdateItemInput
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
		updateRequest.UpdateItemInput,
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

type TransactionWriteItemOptions interface {
	applyTransactionWriteItemOption(cfg *TransactionWriteItemConfig)
}

type TransactionWriteItemConfig struct {
}

func prepareWriteTransactionRequest(
	writeRequests []TransactionWriteItemRequest,
	encoder *serializer.Encoder,
) (request *dynamodb.TransactWriteItemsInput, err error) {
	var txRequests []types.TransactWriteItem
	for _, writeReq := range writeRequests {
		for _, checkReq := range writeReq.Checks {
			conditionCheck, err := prepareTransactionCheckRequest(writeReq.Table, checkReq)
			if err != nil {
				return nil, err
			}
			txRequests = append(txRequests,
				types.TransactWriteItem{
					ConditionCheck: conditionCheck,
				},
			)
		}
		for _, updateReq := range writeReq.Updates {
			update, err := prepareTransactionUpdateRequest(writeReq.Table, updateReq, encoder)
			if err != nil {
				return nil, err
			}
			txRequests = append(txRequests,
				types.TransactWriteItem{
					Update: update,
				},
			)
		}
		for _, putReq := range writeReq.Puts {
			put, err := prepareTransactionPutRequest(writeReq.Table, putReq, encoder)
			if err != nil {
				return nil, err
			}
			txRequests = append(txRequests,
				types.TransactWriteItem{
					Put: put,
				},
			)
		}
		for _, deleteReq := range writeReq.Deletes {
			del, err := prepareTransactionDeleteRequest(writeReq.Table, deleteReq)
			if err != nil {
				return nil, err
			}
			txRequests = append(txRequests,
				types.TransactWriteItem{
					Delete: del,
				},
			)
		}
	}
	return &dynamodb.TransactWriteItemsInput{
		TransactItems: txRequests,
	}, nil
}
func TransactionWriteItems(
	ctx context.Context,
	client TransactionWriteItemClient,
	writeRequests []TransactionWriteItemRequest,
	opts ...TransactionWriteItemOptions) (err error) {

	encoder := client.GetEncoder()
	if encoder == nil {
		encoder = s_encoder
	}
	request, err := prepareWriteTransactionRequest(writeRequests, encoder)
	if err != nil {
		return err
	}
	_, err = client.TransactWriteItems(ctx, request)
	if err != nil {
		return err
	}
	return nil
}

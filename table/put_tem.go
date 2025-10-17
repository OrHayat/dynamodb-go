package table

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/serializer"
)

type PutItemClient interface {
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	GetEncoder() *serializer.Encoder
}

type PutItemOptions interface {
	applyPutItemOption(*PutItemConfig)
}

type PutItemConfig struct {
}

func getPutItemExpression(
	conditionalExpression expression.ConditionBuilder,
) (expr expression.Expression, err error) {
	if !conditionalExpression.IsSet() {
		return
	}
	b := expression.NewBuilder()
	b = b.WithCondition(conditionalExpression)
	return b.Build()
}

func preparePutItemRequest(
	table *TableDefinition,
	conditionalExpression expression.ConditionBuilder,
	encodedItem map[string]types.AttributeValue,
) (*dynamodb.PutItemInput, error) {

	expr, err := getPutItemExpression(conditionalExpression)
	if err != nil {
		return nil, err
	}
	request := &dynamodb.PutItemInput{
		TableName:                           &table.Name,
		Item:                                encodedItem,
		ConditionExpression:                 expr.Condition(),
		ExpressionAttributeNames:            expr.Names(),
		ExpressionAttributeValues:           expr.Values(),
		ReturnConsumedCapacity:              "", //TODO: add way to return consumed capacity
		ReturnItemCollectionMetrics:         "", //TODO: add way to return item collection metrics
		ReturnValues:                        "",
		ReturnValuesOnConditionCheckFailure: types.ReturnValuesOnConditionCheckFailureAllOld,
	}
	return request, nil
}

// execute put item request request
//
// if inputinput.AllowReplaceItem is false - ensure item not exists condition will be added to the request
func putOrReplaceItem(
	ctx context.Context,
	client PutItemClient,
	table *TableDefinition,
	input PutItemInput,
	encoder *serializer.Encoder,
) (err error) {

	encodedItem, err := serializer.MarshalMap(encoder, input.Item)
	if err != nil {
		return err
	}
	pk, sk, err := table.ExtractKeys(encodedItem) //ensure PK and SK are present
	if err != nil {
		return &OperationError{
			operation:   "put request validation",
			table:       table,
			internalErr: err,
			pk:          table.encodedKeyToVal(pk),
			sk:          table.encodedKeyToVal(sk),
		}
	}

	request, err := preparePutItemRequest(table, input.ConditionalCheck, encodedItem)
	if err != nil {
		return err
	}

	_, err = client.PutItem(ctx, request)
	if err != nil {
		if !input.AllowReplaceItem.Bool() {
			if _, ok := ErrorAs[*types.ConditionalCheckFailedException](err); ok {
				return &OperationError{
					operation:   "put item",
					table:       table,
					pk:          table.encodedKeyToVal(pk),
					sk:          table.encodedKeyToVal(sk),
					internalErr: ErrAlreadyExists,
				}
			}
		}
		return &OperationError{
			operation:   "put item",
			table:       table,
			internalErr: err,
			pk:          table.encodedKeyToVal(pk),
			sk:          table.encodedKeyToVal(sk),
		}
	}
	return nil
}

// --------------- put or replace item ----------------

type PutItemInput struct {
	//item to put in the database
	Item any
	//if this is true - replace item is allowed
	//otherwise item not exists check will be added to the query
	AllowReplaceItem aws.Ternary
	//additional conditions that can be used to limit the replacement item operation
	//
	//this option requires AllowReplaceItem to be true
	ConditionalCheck expression.ConditionBuilder
}

func PutItem(
	ctx context.Context,
	client PutItemClient,
	table *TableDefinition,
	input PutItemInput,
	opts ...PutItemOptions,
) (err error) {

	if input.AllowReplaceItem == aws.UnknownTernary {
		input.AllowReplaceItem = aws.FalseTernary
	}
	if input.ConditionalCheck.IsSet() && !input.AllowReplaceItem.Bool() {
		return &OperationError{
			table:       table,
			internalErr: errors.New("invalid put item options: conditional check can be used only when AllowReplaceItem is true"),
		}
	}

	encoder := client.GetEncoder()
	if encoder == nil {
		encoder = s_encoder
	}

	if !input.AllowReplaceItem.Bool() {
		input.ConditionalCheck = ensureKeyNotExists(table)
	}
	return putOrReplaceItem(ctx, client, table, input, encoder)
}

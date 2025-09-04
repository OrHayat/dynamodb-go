package table

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type PutItemClient interface {
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...DynamoDBFuncOpts) (*dynamodb.PutItemOutput, error)
}

type PutNewItemQueryOpts struct {
	ReturnValuesOnConditionCheckFailure bool //default true can pass false to save some network incase of itemAlreadyExists error
}

// return query that will insert NEW item to the table - it will fail if the item already exists
func NewPutNewItemInput[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	table Table[KeyType, HashKeyType, RangeKeyType],
	object any,
	optsFns ...FuncOption[PutNewItemQueryOpts],
) (queryInput dynamodb.PutItemInput, err error) {

	opts := PutNewItemQueryOpts{
		ReturnValuesOnConditionCheckFailure: true,
	}

	ApplyOptions(&opts, optsFns)

	return newPutItemQuey(
		table,
		object,
		putItemOpts{
			allowReplaceExistingItem:            false,
			conditionalBuilder:                  expression.ConditionBuilder{}, //none
			returnValuesOnConditionCheckFailure: opts.ReturnValuesOnConditionCheckFailure,
			returnOldValue:                      false, //nothing to return... new item
		})

}

type PutNewOrReplaceItemQueryOpts struct {
	ReturnValuesOnConditionCheckFailure bool
	ReturnOldValue                      bool
	ConditionalBuilder                  expression.ConditionBuilder
}

// put the item - will replace exists item
func NewPutNewOrReplaceItemInput[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	table Table[KeyType, HashKeyType, RangeKeyType],
	object any,
	optsFns ...FuncOption[PutNewOrReplaceItemQueryOpts],
) (queryInput dynamodb.PutItemInput, err error) {

	opts := PutNewOrReplaceItemQueryOpts{
		ReturnValuesOnConditionCheckFailure: true,
		ReturnOldValue:                      true,
	}

	ApplyOptions(&opts, optsFns)

	return newPutItemQuey(
		table,
		object,
		putItemOpts{
			allowReplaceExistingItem:            true,
			conditionalBuilder:                  opts.ConditionalBuilder,
			returnValuesOnConditionCheckFailure: opts.ReturnValuesOnConditionCheckFailure,
			returnOldValue:                      opts.ReturnOldValue,
		})
}

type putItemOpts struct {
	allowReplaceExistingItem            bool
	conditionalBuilder                  expression.ConditionBuilder
	returnValuesOnConditionCheckFailure bool
	returnOldValue                      bool
}

// TODO: add required attributes to the encoder and validate it on here
//
// internal helper function shared between putNewItem and put
func newPutItemQuey[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	table Table[KeyType, HashKeyType, RangeKeyType],
	object any,
	opts putItemOpts,
) (queryInput dynamodb.PutItemInput, err error) {

	//encode the item
	encodedItem, err := attributevalue.MarshalMapWithOptions(object,
		func(eo *attributevalue.EncoderOptions) {
			eo.EncodeTime = encodeTimeRfc339
		},
	)
	if err != nil {
		return queryInput, err
	}

	//TODO:validate schema somehow?
	key, err := table.KeyEncoder.ExtractKey(encodedItem)
	if err != nil {
		return queryInput, err
	}

	queryInput.TableName = aws.String(table.TableName)
	queryInput.Item = encodedItem

	//optionally return the attribute on conditional check failure
	if opts.returnValuesOnConditionCheckFailure {
		queryInput.ReturnValuesOnConditionCheckFailure = types.ReturnValuesOnConditionCheckFailureAllOld
	} else {
		queryInput.ReturnValuesOnConditionCheckFailure = types.ReturnValuesOnConditionCheckFailureNone
	}

	//return old value
	if opts.returnOldValue {
		queryInput.ReturnValues = types.ReturnValueAllOld
	} else {
		queryInput.ReturnValues = types.ReturnValueNone
	}

	conditionalBuilder := opts.conditionalBuilder
	if !opts.allowReplaceExistingItem {
		conditionalBuilder = addAttributesDontExistsToQueryBuilder(conditionalBuilder, key)
	}

	if conditionalBuilder.IsSet() {
		builder := expression.NewBuilder()
		builder = builder.WithCondition(conditionalBuilder)
		expr, err := builder.Build()
		if err != nil {
			return queryInput, fmt.Errorf("failed to build expression for put item query table %s : %w", table.TableName, err)
		}
		queryInput.ConditionExpression = expr.Condition()
		queryInput.ExpressionAttributeNames = expr.Names()
		queryInput.ExpressionAttributeValues = expr.Values()
	}

	return queryInput, nil
}

func ExecutePutNewItem(
	ctx context.Context,
	client PutItemClient,
	input *dynamodb.PutItemInput,
	optsFns ...DynamoDBFuncOpts,
) (err error) {
	_, err = client.PutItem(ctx, input, optsFns...)
	if err != nil {
		var conditionalCheckErr *types.ConditionalCheckFailedException
		if errors.As(err, &conditionalCheckErr) && conditionalCheckErr.Item != nil {
			return newErrorAlreadyExists(*input.TableName, conditionalCheckErr.Item)
		}
		return err
	}
	return nil
}

type PutNewItemOpts struct {
	QueryOpts  []FuncOption[PutNewItemQueryOpts]
	DynamoOpts []DynamoDBFuncOpts
}

// put new item in dynamo db table
func PutNewItem[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	ctx context.Context,
	client PutItemClient,
	table Table[KeyType, HashKeyType, RangeKeyType],
	object KeyType,
	optsFns ...FuncOption[PutNewItemOpts],
) (err error) {

	opts := PutNewItemOpts{}
	ApplyOptions(&opts, optsFns)

	input, err := NewPutNewItemInput(table, object, opts.QueryOpts...)
	if err != nil {
		return err
	}

	return ExecutePutNewItem(ctx, client, &input, opts.DynamoOpts...)
}

// *** warning *** caller might want to pass map[string]any instead of struct to avoid unmarshal errors incase old item has different spec
func ExecutePutNewOrReplaceItem[T any](
	ctx context.Context,
	client PutItemClient,
	input *dynamodb.PutItemInput,
	optsFns ...DynamoDBFuncOpts,
) (oldItem *T, err error) { //TODO: replace *T with optional[T]?

	input.ReturnValues = types.ReturnValueAllOld
	output, err := client.PutItem(ctx, input, optsFns...)

	if err != nil {
		return oldItem, err
	}

	if output.Attributes == nil {
		return oldItem, nil
	}

	res, err := UnmarshalMap[T](output.Attributes)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

type PutNewOrReplaceItemOpts struct {
	QueryOpts  []FuncOption[PutNewOrReplaceItemQueryOpts]
	DynamoOpts []DynamoDBFuncOpts
}

// *** warning *** caller might want to pass map[string]any instead of struct to avoid unmarshal errors incase old item has different spec
func PutNewOrReplaceItem[
	ItemType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	ctx context.Context,
	client PutItemClient,
	table Table[ItemType, HashKeyType, RangeKeyType],
	object ItemType,
	optsFns ...FuncOption[PutNewOrReplaceItemOpts],
) (oldItem *ItemType, err error) { //TODO: replace with optional[ItemType]

	opts := PutNewOrReplaceItemOpts{}
	ApplyOptions(&opts, optsFns)

	input, err := NewPutNewOrReplaceItemInput(table, object, opts.QueryOpts...)
	if err != nil {
		return nil, err
	}

	return ExecutePutNewOrReplaceItem[ItemType](ctx, client, &input, opts.DynamoOpts...)
}

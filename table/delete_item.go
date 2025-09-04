package table

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DeleteItemClient interface {
	DeleteItem(ctx context.Context, input *dynamodb.DeleteItemInput, opts ...DynamoDBFuncOpts) (*dynamodb.DeleteItemOutput, error)
}

type DeleteItemInputForTableOpts struct {
	ConditionalExpressionBuilder expression.ConditionBuilder //optional conditions for the delete
	//if true will return  DeleteRequestItemDontExistsHandler that will need to be passed to ExecuteDeleteRequest
	//
	//to make sure to return ErrItemDoesnt exists  if false dynamodb will not return error if the item doesnt exists in the database
	CheckForItemDoesntExistsErr bool
}

func NewDeleteItemInputForTable[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	table Table[KeyType, HashKeyType, RangeKeyType],
	key KeyType,
	optsFns ...FuncOption[DeleteItemInputForTableOpts],
) (queryInput dynamodb.DeleteItemInput, errHandler DeleteRequestItemDontExistsHandler, err error) {

	opts := DeleteItemInputForTableOpts{
		CheckForItemDoesntExistsErr: true,
	}

	ApplyOptions(&opts, optsFns)

	keyAttributes, err := table.KeyEncoder.EncodeKey(key)
	if err != nil {
		return queryInput, nil, fmt.Errorf("failed table %s to encode key %#v :%w", table.TableName, key, err)
	}

	queryInput.TableName = aws.String(table.TableName)
	queryInput.Key = keyAttributes
	if opts.CheckForItemDoesntExistsErr {
		//if passed ConditionalExpressionBuilder need to ask dynamodb to return the item+check the response if the item was deleted by this request
		if opts.ConditionalExpressionBuilder.IsSet() {
			errHandler = deleteRequestItemDoesntExistsResponseChecker{}
		} else {
			//there is not ConditionalExpressionBuilder can use more efficient way to make dynamodb return error if the item not exists
			//by adding condition to the query to make it fail if item doesnt exists
			//because it the only condition in the query then the ConditionalCheckFailures can be transformed into ItemDoesntExists error
			errHandler = deleteRequestItemDoesntExistsConditionChecker{}
			opts.ConditionalExpressionBuilder = addAttributesExistsToQueryBuilder(opts.ConditionalExpressionBuilder, keyAttributes)
		}
	}

	if opts.ConditionalExpressionBuilder.IsSet() {
		builder := expression.NewBuilder()
		builder = builder.WithCondition(opts.ConditionalExpressionBuilder)
		expr, err := builder.Build()
		if err != nil {
			return queryInput, errHandler, fmt.Errorf("failed to build expression for delete item query table %s : %w", table.TableName, err)
		}

		queryInput.ExpressionAttributeNames = expr.Names()
		queryInput.ExpressionAttributeValues = expr.Values()
		queryInput.ConditionExpression = expr.Condition()
	}
	return queryInput, errHandler, nil
}

type DeleteRequestItemDontExistsHandler interface {
	PrepareInput(input *dynamodb.DeleteItemInput) (err error)                      //modify the input if needed to give extra information return error if not possible
	IsItemDoesntExistsErr(*dynamodb.DeleteItemOutput, error) (isErrNotExists bool) //true if the item doesnt exists,error will be not nil if the handle failed to check the item/error for any reason
}

var _ DeleteRequestItemDontExistsHandler = deleteRequestItemDoesntExistsConditionChecker{}

type deleteRequestItemDoesntExistsConditionChecker struct{}

func (deleteRequestItemDoesntExistsConditionChecker) PrepareInput(input *dynamodb.DeleteItemInput) error {
	return nil
}

func (deleteRequestItemDoesntExistsConditionChecker) IsItemDoesntExistsErr(output *dynamodb.DeleteItemOutput, err error) (isErrNotExists bool) {
	if err != nil {
		var conditionalCheckErr *types.ConditionalCheckFailedException
		if errors.As(err, &conditionalCheckErr) {
			return true
		}
	}
	return false
}

var _ DeleteRequestItemDontExistsHandler = deleteRequestItemDoesntExistsResponseChecker{}

// check that the deleted item doesnt exists by asking dynamodb to return the deleted item
//
// incase the error is nil the output expected to contain the deleted item
//
// if the deleted item is not in the return result it mean that the item doesnt exists
type deleteRequestItemDoesntExistsResponseChecker struct{}

// prepareInput implements deleteRequestItemDontExistsHandler.
func (deleteRequestItemDoesntExistsResponseChecker) PrepareInput(input *dynamodb.DeleteItemInput) (err error) {
	input.ReturnValues = types.ReturnValueAllOld
	return nil
}

func (deleteRequestItemDoesntExistsResponseChecker) IsItemDoesntExistsErr(output *dynamodb.DeleteItemOutput, err error) (isErrNotExists bool) {
	if err == nil && output != nil {
		if output.Attributes == nil {
			return true
		}
	}
	return false
}

func executeItemDeleteInternal(
	ctx context.Context,
	client DeleteItemClient,
	input *dynamodb.DeleteItemInput,
	itemDoesntExistsChecker DeleteRequestItemDontExistsHandler, //TODO: make optional
	optsFns ...DynamoDBFuncOpts,
) (oldItem map[string]types.AttributeValue, err error) {

	if itemDoesntExistsChecker != nil {
		err = itemDoesntExistsChecker.PrepareInput(input)
		if err != nil {
			return nil, err
		}
	}

	output, err := client.DeleteItem(ctx, input, optsFns...)
	if itemDoesntExistsChecker != nil {
		if itemDoesntExistsChecker.IsItemDoesntExistsErr(output, err) {
			err = newErrorNotExists(*input.TableName, input.Key, err)
		}
	}

	if err != nil {
		return nil, err
	}

	return output.Attributes, nil
}

func ExecuteDeleteItem(
	ctx context.Context,
	client DeleteItemClient,
	input *dynamodb.DeleteItemInput,
	itemDoesntExistsChecker DeleteRequestItemDontExistsHandler, //TODO: make optional
	optsFns ...DynamoDBFuncOpts,
) (err error) {

	_, err = executeItemDeleteInternal(ctx, client, input, itemDoesntExistsChecker, optsFns...)
	return err
}

func ExecuteDeleteItemAndReturnOldValue[T any](
	ctx context.Context,
	client DeleteItemClient,
	input *dynamodb.DeleteItemInput,
	itemDoesntExistsChecker DeleteRequestItemDontExistsHandler, //TODO: make optional
	optsFns ...DynamoDBFuncOpts,
) (item *T, err error) {

	//force input to return old values - even if it was not requested by user
	input.ReturnValues = types.ReturnValueAllOld
	oldAttrs, err := executeItemDeleteInternal(ctx, client, input, itemDoesntExistsChecker, optsFns...)
	if err != nil {
		return nil, err
	}

	res, err := UnmarshalMap[T](oldAttrs)
	if err != nil {
		return nil, err
	}
	item = &res

	return item, nil
}

type DeleteItemOpts struct {
	QueryOpts       []FuncOption[DeleteItemInputForTableOpts]
	DynamoDBOptions []DynamoDBFuncOpts
}

// simple function to create query+delete the item at 1 call instead of 2
func DeleteItem[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	ctx context.Context,
	client DeleteItemClient,
	table Table[KeyType, HashKeyType, RangeKeyType],
	key KeyType,
	optsFns ...FuncOption[DeleteItemOpts],
) (err error) {

	opts := DeleteItemOpts{}
	ApplyOptions(&opts, optsFns)

	input, itemDoesntExistsChecker, err := NewDeleteItemInputForTable(table, key, opts.QueryOpts...)
	if err != nil {
		return err
	}
	return ExecuteDeleteItem(ctx, client, &input, itemDoesntExistsChecker, opts.DynamoDBOptions...)
}

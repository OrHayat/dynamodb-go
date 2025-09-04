package table

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type GetItemClient interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...DynamoDBFuncOpts) (*dynamodb.GetItemOutput, error)
}

type GetItemQueryOpts struct {
	ProjectionBuilder *expression.ProjectionBuilder
	ConsistentRead    ConsistencyInputType
}

func NewGetItemInputForTable[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	table Table[KeyType, HashKeyType, RangeKeyType],
	key KeyType,
	optsFns ...FuncOption[GetItemQueryOpts],
) (queryInput dynamodb.GetItemInput, err error) {

	opts := GetItemQueryOpts{
		ConsistentRead: ConsistentRead,
	}

	ApplyOptions(&opts, optsFns)

	keyAttributes, err := table.KeyEncoder.EncodeKey(key)
	if err != nil {
		return queryInput, fmt.Errorf("failed table %s to encode key %#v :%w", table.TableName, key, err)
	}

	queryInput.TableName = aws.String(table.TableName)
	queryInput.Key = keyAttributes
	queryInput.ConsistentRead = aws.Bool(opts.ConsistentRead)

	if opts.ProjectionBuilder != nil {
		builder := expression.NewBuilder().WithProjection(*opts.ProjectionBuilder)
		exp, err := builder.Build()
		if err != nil {
			return queryInput, err
		}
		queryInput.ProjectionExpression = exp.Projection()
		queryInput.ExpressionAttributeNames = exp.Names()
	}

	return queryInput, nil
}

func ExecuteGetItem[T any](
	ctx context.Context,
	client GetItemClient,
	input *dynamodb.GetItemInput,
	optFns ...func(*dynamodb.Options),
) (item T, err error) {

	out, err := client.GetItem(ctx, input, optFns...)
	if err != nil {
		return item, fmt.Errorf("failed to get item %#v from table %s", input.Key, *input.TableName)
	}

	if out.Item == nil {
		return item, newErrorNotExists(*input.TableName, input.Key, nil)
	}

	item, err = UnmarshalMap[T](out.Item)
	if err != nil {
		return item, err
	}

	return item, nil
}

type GetItemFromTableOpts struct {
	QueryOptions    []FuncOption[GetItemQueryOpts]
	DynamoDBOptions []DynamoDBFuncOpts
}

func GetItemFromTable[
	ItemType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	ctx context.Context,
	client GetItemClient,
	table Table[ItemType, HashKeyType, RangeKeyType],
	key ItemType,
	optsFns ...FuncOption[GetItemFromTableOpts],
) (item ItemType, err error) {

	opts := GetItemFromTableOpts{}
	ApplyOptions(&opts, optsFns)

	input, err := NewGetItemInputForTable(table, key, opts.QueryOptions...)
	if err != nil {
		return item, err
	}

	return ExecuteGetItem[ItemType](ctx, client, &input, opts.DynamoDBOptions...)
}

package table

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/orhayat/dynamodb-go/serializer"
)

type GetItemClient interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	GetDecoder() *serializer.Decoder //mil imply to use default decoder
}

type GetItemConfig struct {
}

type GetItemOptions interface {
	applyGetItemOption(*GetItemConfig)
}
type GetItemInput struct {
	Key            Key
	ConsistentRead aws.Ternary
	Projection     *expression.ProjectionBuilder
}

func prepareGetExpression(
	projection *expression.ProjectionBuilder,
) (expr expression.Expression, err error) {
	if projection == nil {
		return
	}
	b := expression.NewBuilder()
	b = b.WithProjection(*projection)
	return b.Build()
}

func prepareGetRequest(
	tableName string,
	projection *expression.ProjectionBuilder,
	consistentRead aws.Ternary,
	key map[string]types.AttributeValue,
) (*dynamodb.GetItemInput, error) {

	expr, err := prepareGetExpression(projection)
	if err != nil {
		return nil, err
	}

	return &dynamodb.GetItemInput{
		TableName:                &tableName,
		Key:                      key,
		ProjectionExpression:     expr.Projection(),
		ExpressionAttributeNames: expr.Names(),
		ConsistentRead:           aws.Bool(consistentRead.Bool()),
		ReturnConsumedCapacity:   "", //TODO: add way to return consumed capacity
	}, nil
}

func GetItem(
	ctx context.Context,
	client GetItemClient,
	table *TableDefinition,
	input GetItemInput,
	out any,
	opts ...GetItemOptions,
) (err error) {

	decoder := client.GetDecoder()
	if decoder == nil {
		decoder = s_decoder
	}

	encodedKey, err := table.getKey(input.Key)
	if err != nil {
		return &OperationError{
			operation:   "get item key encoding",
			table:       table,
			pk:          input.Key.PK,
			sk:          input.Key.SK,
			internalErr: err,
		}
	}

	request, err := prepareGetRequest(table.Name, input.Projection, input.ConsistentRead, encodedKey)
	if err != nil {
		return &OperationError{
			operation:   "get item prepare request",
			table:       table,
			pk:          input.Key.PK,
			sk:          input.Key.SK,
			internalErr: err,
		}
	}
	res, err := client.GetItem(ctx, request)
	if err != nil {
		return &OperationError{
			operation:   "get item",
			table:       table,
			pk:          input.Key.PK,
			sk:          input.Key.SK,
			internalErr: err,
		}
	}

	if len(res.Item) == 0 {
		return &OperationError{
			operation:   "get item",
			table:       table,
			pk:          input.Key.PK,
			sk:          input.Key.SK,
			internalErr: ErrItemNotFound,
		}
	}

	err = serializer.UnmarshalMap(decoder, res.Item, out)
	if err != nil {
		return &OperationError{
			operation:   "get item unmarshal",
			table:       table,
			pk:          input.Key.PK,
			sk:          input.Key.SK,
			internalErr: err,
		}
	}
	return nil
}

func GetItemOf[T any](
	ctx context.Context,
	client GetItemClient,
	table *TableDefinition,
	input GetItemInput,
	opts ...GetItemOptions,
) (out T, err error) {
	err = GetItem(ctx, client, table, input, &out, opts...)
	return
}

func GetAsJSON(
	ctx context.Context,
	client GetItemClient,
	table *TableDefinition,
	input GetItemInput,
	opts ...GetItemOptions,
) (out map[string]any, err error) {
	var res any
	err = GetItem(ctx, client, table, input, &res, opts...)
	if err != nil {
		return nil, err
	}
	return res.(map[string]any), nil
}

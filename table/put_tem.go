package table

import (
	"context"

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
	Encoder *serializer.Encoder
}

func putRequestPrepareConditionalCheck(
	table *TableDefinition,
	cfg *PutItemConfig,
	upsert bool,
) (cond expression.ConditionBuilder) {
	if !upsert {
		cond = expression.AttributeNotExists(expression.Name(table.PrimaryKey.Name))
		if table.RangeKey.Name != "" {
			cond = cond.And(expression.AttributeNotExists(expression.Name(table.RangeKey.Name)))
		}
		return cond
	}
	return cond
}

func preparePutItemRequest(
	table *TableDefinition,
	conditionalExpression expression.ConditionBuilder,
	encodedItem map[string]types.AttributeValue,
) (*dynamodb.PutItemInput, error) {

	var names map[string]string
	var values map[string]types.AttributeValue
	var condition *string

	if conditionalExpression.IsSet() {
		b := expression.NewBuilder()
		b = b.WithCondition(conditionalExpression)
		var err error
		expr, err := b.Build()
		if err != nil {
			return nil, err
		}
		names = expr.Names()
		values = expr.Values()
		condition = expr.Condition()
	}

	request := &dynamodb.PutItemInput{
		TableName:                           &table.Name,
		Item:                                encodedItem,
		ConditionExpression:                 condition,
		ExpressionAttributeNames:            names,
		ExpressionAttributeValues:           values,
		ReturnConsumedCapacity:              "", //TODO: add way to return consumed capacity
		ReturnItemCollectionMetrics:         "", //TODO: add way to return item collection metrics
		ReturnValues:                        "",
		ReturnValuesOnConditionCheckFailure: types.ReturnValuesOnConditionCheckFailureAllOld,
	}
	return request, nil
}

func putOrUpsertItem(
	ctx context.Context,
	client PutItemClient,
	table *TableDefinition,
	item any,
	encoder *serializer.Encoder,
	conditionalExpression expression.ConditionBuilder,
	upsert bool,
) (err error) {
	encodedItem, err := serializer.MarshalMap(encoder, item)
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
	request, err := preparePutItemRequest(table, conditionalExpression, encodedItem)
	if err != nil {
		return err
	}

	_, err = client.PutItem(ctx, request)
	if err != nil {
		if !upsert {
			//check if need to return item not exists.
			if _, ok := ErrorAs[*types.ConditionalCheckFailedException](err); ok {
				return &OperationError{
					operation:   "delete item",
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

// put new item on the table
// if item already exists function will fail
// to allow replacing of existing use UpsertItem function
func PutItem(
	ctx context.Context,
	client PutItemClient,
	table *TableDefinition,
	item any,
	opts ...PutItemOptions,
) (err error) {
	cfg := &PutItemConfig{
		Encoder: nil,
	}
	for _, opt := range opts {
		opt.applyPutItemOption(cfg)
	}
	if cfg.Encoder == nil {
		cfg.Encoder = client.GetEncoder()
	}
	if cfg.Encoder == nil {
		cfg.Encoder = s_encoder
	}
	conditionExpression := expression.AttributeNotExists(expression.Name(table.PrimaryKey.Name))
	if table.RangeKey.Name != "" {
		conditionExpression = conditionExpression.And(expression.AttributeNotExists(expression.Name(table.RangeKey.Name)))
	}
	return putOrUpsertItem(ctx, client, table, item, cfg.Encoder, conditionExpression, false)
}

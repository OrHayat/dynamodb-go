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

func preparePutItemRequest(
	table *TableDefinition,
	cfg *PutItemConfig,
	encodedItem map[string]types.AttributeValue,
) (request *dynamodb.PutItemInput, err error) {
	b := expression.NewBuilder()
	var needBuild bool = false

	if !cfg.AllowUpsert {
		conditionExpression := expression.AttributeNotExists(expression.Name(table.PrimaryKey.Name))
		if table.RangeKey.Name != "" {
			conditionExpression = conditionExpression.And(expression.AttributeNotExists(expression.Name(table.RangeKey.Name)))
		}
		b = b.WithCondition(conditionExpression)
		needBuild = true
	}

	request = &dynamodb.PutItemInput{
		TableName:                           &table.Name,
		Item:                                encodedItem,
		ConditionExpression:                 nil,
		ExpressionAttributeNames:            nil,
		ExpressionAttributeValues:           nil,
		ReturnConsumedCapacity:              "", //safe default- usefull for metrics but this package dont help to export metricss
		ReturnItemCollectionMetrics:         "", //can be used to notify/log collections that are close to getting
		ReturnValues:                        "", //
		ReturnValuesOnConditionCheckFailure: "", //
	}

	if needBuild {
		expr, err := b.Build()
		if err != nil {
			return nil, err
		}
		request.ConditionExpression = expr.Condition()
		request.ExpressionAttributeNames = expr.Names()
		request.ExpressionAttributeValues = expr.Values()
	}
	return request, nil
}

type PutItemConfig struct {
	Encoder *serializer.Encoder
	//default is false and incase item existing it will fail the request
	// - if true, will allow overwriting existing items with same PK/SK
	AllowUpsert bool
}

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
	encodedItem, err := serializer.MarshalMap(cfg.Encoder, item)
	if err != nil {
		return &OperationError{
			operation:   "put item encoding",
			table:       table,
			internalErr: err,
		}
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
	request, err := preparePutItemRequest(table, cfg, encodedItem)
	if err != nil {
		return &OperationError{
			operation:   "put request preparation",
			table:       table,
			internalErr: err,
			pk:          table.encodedKeyToVal(pk),
			sk:          table.encodedKeyToVal(sk),
		}
	}

	_, err = client.PutItem(ctx, request)
	if err != nil {
		if _, ok := ErrorAs[*types.ConditionalCheckFailedException](err); ok {
			return &OperationError{
				operation:   "delete item",
				table:       table,
				pk:          table.encodedKeyToVal(pk),
				sk:          table.encodedKeyToVal(sk),
				internalErr: ErrAlreadyExists,
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

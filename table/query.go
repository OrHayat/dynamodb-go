package table

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/orhayat/dynamodb-go/serializer"
)

type QueryItemsClient interface {
	Query(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	GetDecoder() *serializer.Decoder
}

type QueryOptions interface {
	applyQueryOption(*QueryConfig)
}

type ScanKeysOrder int

const (
	ScanKeysOrderUndefined ScanKeysOrder = iota
	ScanKeysOrderAscending
	ScanKeysOrderDescending
)

type QueryConfig struct {
	ConsistentRead bool
	Decoder        *serializer.Decoder
	Limit          *int32
	ScanKeysOrder  ScanKeysOrder
}

func prepareQueryRequest(
	table *TableDefinition,
	indexName string,
	cfg QueryConfig,
) (*dynamodb.QueryInput, error) {
	var index *string
	if indexName != "" {
		index = aws.String(indexName)
	}
	b := expression.NewBuilder()
	if cfg.ConsistentRead && indexName != "" {
		if table.getLSI(indexName) != nil {
			return nil, fmt.Errorf("cannot use consistent read when querying LSI index %s", indexName)
		}
	}
	exp, err := b.Build()
	if err != nil {
		return nil, &OperationError{
			operation:   "query prepare",
			table:       table,
			internalErr: err,
		}
	}
	request := &dynamodb.QueryInput{
		TableName:                 aws.String(table.Name),
		ConsistentRead:            aws.Bool(cfg.ConsistentRead), //TODO
		ExclusiveStartKey:         nil,                          //TODO: for pagination support
		ExpressionAttributeNames:  exp.Names(),                  ///for filtering/projection
		ExpressionAttributeValues: exp.Values(),                 ///for filtering/projection
		FilterExpression:          nil,                          //need to add way to filter results server side - expose it in api
		KeyConditionExpression:    exp.KeyCondition(),           //need to add way to have more conditions on query - e.g. between, begins_with, et
		ProjectionExpression:      nil,                          //TODO: add way to support projection expression in api
		IndexName:                 index,
		Limit:                     nil,
		ReturnConsumedCapacity:    "",
		ScanIndexForward:          nil, //forward/backward scan order
		Select:                    "",
	}

	return request, nil
}

// // func (t *TableImpl) GetByIndex(ctx context.Context, def *TableDefinition, partitionKey any, attributes Attribute, out any) (err error) {

// // 	conditions := expression.And(
// // 		expression.Name(def.PartitionKey.Name).Equal(expression.Value(partitionKey)),
// // 		expression.Name(attributes.Name).Equal(expression.Value(attributes.Value)))

// // simplest possible Query - by partition key only
// func QueryByPartitionKey(
// 	ctx context.Context,
// 	client QueryItemsClient,
// 	table *TableDefinition,
// 	pk any,
// 	indexName string,
// 	out any,
// ) (err error) {
// 	// request, err := prepareQueryRequest(table, indexName)
// 	// response, err := client.Query(ctx, nil)

// 	// len(response.Items)
// 	return
// }

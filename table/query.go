package table

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/orhayat/dynamodb-go/serializer"
)

type ScanKeysOrder int

const (
	ScanKeysOrderUndefined ScanKeysOrder = iota
	ScanKeysOrderAscending
	ScanKeysOrderDescending
)

func (ord ScanKeysOrder) getScanOrder() *bool {
	var scanOrder *bool
	switch ord {
	case ScanKeysOrderDescending:
		scanOrder = aws.Bool(false)
	default:
		scanOrder = aws.Bool(true)
	}
	return scanOrder
}

type QueryItemsClient interface {
	Query(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	GetDecoder() *serializer.Decoder
}

type QueryOptions interface {
	applyQueryOption(*QueryConfig)
}

type QueryConfig struct {
	Decoder *serializer.Decoder
}

// Validate consistent read usage with GSI
// Consistent reads are not supported on global secondary indexes.
func validateGsiConsistentReadUsage(table *TableDefinition, indexName string, consistentRead bool) error {
	if consistentRead && indexName != "" {
		if table.getGSI(indexName) != nil {
			return &OperationError{
				operation:   "query prepare",
				table:       table,
				index:       indexName,
				internalErr: fmt.Errorf("cannot use consistent read when querying LSI index %s", indexName),
			}
		}
	}
	return nil
}

func prepareKeyConditionForQuery(table *TableDefinition, input QueryInput) (cond expression.KeyConditionBuilder, err error) {
	defer func() {
		if err != nil {
			err = &OperationError{
				operation:   "query prepare",
				table:       table,
				index:       input.Index,
				internalErr: err,
			}
		}
	}()
	key := input.Key
	if key.SK != nil && input.SortKeyCondition.IsSet() {
		return cond, fmt.Errorf("cannot set both range key value and range key condition")
	}
	if key.PK == nil {
		return cond, fmt.Errorf("partition key value must be provided")
	}
	av, err := table.getPkForIndex(input.Index, key.PK)
	if err != nil {
		return cond, fmt.Errorf("failed to encode partition key value: %w", err)
	}
	pkName, err := table.getPkName(input.Index)
	if err != nil {
		return cond, err
	}
	cond = expression.Key(pkName).Equal(expression.Value(av))
	if input.SortKeyCondition.IsSet() {
		cond = cond.And(input.SortKeyCondition)
	} else if key.SK != nil {
		av, err := table.getSkForIndex(input.Index, key.SK)
		if err != nil {
			return cond, fmt.Errorf("failed to encode range key value: %w", err)
		}
		if av != nil {
			skName, err := table.getSkName(input.Index)
			if err != nil {
				return cond, err
			}
			cond = cond.And(expression.Key(skName).Equal(expression.Value(av)))
		}
	}
	return cond, nil
}

func prepareQueryRequest(
	table *TableDefinition,
	input QueryInput,
	cfg QueryConfig,
) (*dynamodb.QueryInput, error) {

	keyCond, err := prepareKeyConditionForQuery(table, input) // indexName, key, cfg)
	if err != nil {
		return nil, err
	}
	var index *string
	if input.Index != "" {
		index = aws.String(input.Index)
	}
	startFrom, err := input.PaginationKey.resolveExclusiveStartKey(table, input.Index)
	if err != nil {
		return nil, &OperationError{
			operation:   "query prepare",
			table:       table,
			index:       input.Index,
			internalErr: err,
		}
	}
	err = validateGsiConsistentReadUsage(table, input.Index, input.ConsistentRead.Bool())
	if err != nil {
		return nil, err
	}

	b := expression.NewBuilder()
	b = b.WithKeyCondition(keyCond)
	if input.FilterExpression.IsSet() {
		b = b.WithFilter(input.FilterExpression)
	}
	if input.ProjectionExpression != nil {
		b = b.WithProjection(*input.ProjectionExpression)
	}
	exp, err := b.Build()
	if err != nil {
		return nil, &OperationError{
			operation:   "query prepare",
			table:       table,
			index:       input.Index,
			internalErr: err,
		}
	}
	var limit *int32
	if input.Limit > 0 {
		limit = aws.Int32(input.Limit)
	}
	request := &dynamodb.QueryInput{
		TableName:                 aws.String(table.Name),
		ConsistentRead:            aws.Bool(input.ConsistentRead.Bool()),
		ExclusiveStartKey:         startFrom,
		ExpressionAttributeNames:  exp.Names(),  //used to  support reserved words in filter and key condition expression,projection expression
		ExpressionAttributeValues: exp.Values(), //values of ExpressionAttributeNames
		FilterExpression:          exp.Filter(), //for filtering on server side
		KeyConditionExpression:    exp.KeyCondition(),
		ProjectionExpression:      exp.Projection(),
		IndexName:                 index,
		Limit:                     limit,
		ReturnConsumedCapacity:    "",
		ScanIndexForward:          input.ScanKeysOrder.getScanOrder(),
		Select:                    "",
	}

	return request, nil
}

type QueryInput struct {
	Key                  Key                            //partion key value is required to be filled, range key is optional , if WithKeyCondition is used in options then range key is not allowed to be set
	SortKeyCondition     expression.KeyConditionBuilder //optional condition on the range key  //cannot be used if range key value is set in the Key struct
	Index                string                         //index to query from - pass empty string to query main table
	PaginationKey        PaginationKey                  //from what key to start the query pagination - pass empty struct to start from beginning of the queried table/index
	ConsistentRead       aws.Ternary
	FilterExpression     expression.ConditionBuilder //optional filter expression to filter results on server side
	ScanKeysOrder        ScanKeysOrder
	Limit                int32
	ProjectionExpression *expression.ProjectionBuilder
}

func Query(
	ctx context.Context,
	client QueryItemsClient,
	table *TableDefinition,
	input QueryInput,
	out any,
	opts ...QueryOptions,
) (nextPage PaginationKey, err error) {
	cfg := QueryConfig{
		Decoder: nil,
	}
	for _, opt := range opts {
		opt.applyQueryOption(&cfg)
	}
	if cfg.Decoder == nil {
		cfg.Decoder = client.GetDecoder()
	}
	if cfg.Decoder == nil {
		cfg.Decoder = s_decoder
	}

	request, err := prepareQueryRequest(table, input, cfg)
	if err != nil {
		return nextPage, err
	}
	indexName := input.Index
	response, err := client.Query(ctx, request)
	if err != nil {
		return nextPage, &OperationError{
			operation:   "query",
			table:       table,
			index:       indexName,
			internalErr: err,
		}
	}

	err = serializer.UnmarshalListOfMaps(cfg.Decoder, response.Items, out)
	if err != nil {
		return nextPage, &OperationError{
			operation:   "query decode",
			table:       table,
			index:       indexName,
			internalErr: err,
		}
	}
	nextPage = PaginationKey{
		encodedKey: response.LastEvaluatedKey,
	}
	return nextPage, nil
}

//TODO: enable QueryOf after finalizing api
// func QueryOf[T any](
// 	ctx context.Context,
// 	client QueryItemsClient,
// 	table *TableDefinition,
// 	key Key, //partion key value is required to be filled, range key is optional , if WithKeyCondition is used in options then range key is not allowed to be set
// 	indexName string, //pass empty string to query main table
// 	paginationKey PaginationKey, //from what key to start the query pagination - pass empty struct to start from beginning of the queried table/index
// 	opts ...QueryOptions,
// ) (results []T, nextPage PaginationKey, err error) {
// 	nextPage, err = Query(ctx, client, table, key, indexName, paginationKey, &results, opts...)
// 	if err != nil {
// 		return nil, nextPage, err
// 	}
// 	return results, nextPage, nil
// }

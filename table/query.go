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

type QueryConfig struct {
	ConsistentRead   bool
	Decoder          *serializer.Decoder
	Limit            *int32
	FilterExpression expression.ConditionBuilder
	ScanKeysOrder    ScanKeysOrder
	SortKeyCondition expression.KeyConditionBuilder
}

// translate ScanKeysOrder to bool pointer used by aws sdk
func (cfg *QueryConfig) getScanOrder() *bool {
	var scanOrder *bool
	switch cfg.ScanKeysOrder {
	case ScanKeysOrderAscending:
		scanOrder = aws.Bool(true)
	case ScanKeysOrderDescending:
		scanOrder = aws.Bool(false)
	}
	return scanOrder
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

func prepareKeyConditionForQuery(table *TableDefinition, indexName string, key Key, cfg QueryConfig) (cond expression.KeyConditionBuilder, err error) {
	defer func() {
		if err != nil {
			err = &OperationError{
				operation:   "query prepare",
				table:       table,
				index:       indexName,
				internalErr: err,
			}
		}
	}()
	if key.SK != nil && cfg.SortKeyCondition.IsSet() {
		return cond, fmt.Errorf("cannot set both range key value and range key condition")
	}
	if key.PK == nil {
		return cond, fmt.Errorf("partition key value must be provided")
	}
	av, err := table.getPkForIndex(indexName, key.PK)
	if err != nil {
		return cond, fmt.Errorf("failed to encode partition key value: %w", err)
	}
	pkName, err := table.getPkName(indexName)
	if err != nil {
		return cond, err
	}
	cond = expression.Key(pkName).Equal(expression.Value(av))
	if cfg.SortKeyCondition.IsSet() {
		cond = cond.And(cfg.SortKeyCondition)
	} else if key.SK != nil {
		av, err := table.getSkForIndex(indexName, key.SK)
		if err != nil {
			return cond, fmt.Errorf("failed to encode range key value: %w", err)
		}
		if av != nil {
			skName, err := table.getSkName(indexName)
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
	indexName string,
	paginationKey PaginationKey,
	key Key,
	cfg QueryConfig,
) (*dynamodb.QueryInput, error) {

	keyCond, err := prepareKeyConditionForQuery(table, indexName, key, cfg)
	if err != nil {
		return nil, err
	}
	var index *string
	if indexName != "" {
		index = aws.String(indexName)
	}
	startFrom, err := paginationKey.resolveExclusiveStartKey(table, indexName)
	if err != nil {
		return nil, &OperationError{
			operation:   "query prepare",
			table:       table,
			index:       indexName,
			internalErr: err,
		}
	}
	err = validateGsiConsistentReadUsage(table, indexName, cfg.ConsistentRead)
	if err != nil {
		return nil, err
	}

	b := expression.NewBuilder()
	b = b.WithKeyCondition(keyCond)
	if cfg.FilterExpression.IsSet() {
		b = b.WithFilter(cfg.FilterExpression)
	}
	exp, err := b.Build()
	if err != nil {
		return nil, &OperationError{
			operation:   "query prepare",
			table:       table,
			index:       indexName,
			internalErr: err,
		}
	}

	request := &dynamodb.QueryInput{
		TableName:                 aws.String(table.Name),
		ConsistentRead:            aws.Bool(cfg.ConsistentRead),
		ExclusiveStartKey:         startFrom,
		ExpressionAttributeNames:  exp.Names(),  //used to  support reserved words in filter and key condition expression,projection expression
		ExpressionAttributeValues: exp.Values(), //values of ExpressionAttributeNames
		FilterExpression:          exp.Filter(), //for filtering on server side
		KeyConditionExpression:    exp.KeyCondition(),
		ProjectionExpression:      nil, //TODO: add way to support projection expression in api
		IndexName:                 index,
		Limit:                     cfg.Limit,
		ReturnConsumedCapacity:    "",
		ScanIndexForward:          cfg.getScanOrder(),
		Select:                    "",
	}

	return request, nil
}

func Query(
	ctx context.Context,
	client QueryItemsClient,
	table *TableDefinition,
	key Key, //partion key value is required to be filled, range key is optional , if WithKeyCondition is used in options then range key is not allowed to be set
	indexName string, //pass empty string to query main table
	paginationKey PaginationKey, //from what key to start the query pagination - pass empty struct to start from beginning of the queried table/index
	out any,
	opts ...QueryOptions,
) (nextPage PaginationKey, err error) {
	cfg := QueryConfig{
		ConsistentRead: false,
		Decoder:        nil,
		Limit:          nil,
		ScanKeysOrder:  ScanKeysOrderAscending,
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

	request, err := prepareQueryRequest(table, indexName, paginationKey, key, cfg)
	if err != nil {
		return nextPage, err
	}
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

func QueryOf[T any](
	ctx context.Context,
	client QueryItemsClient,
	table *TableDefinition,
	key Key, //partion key value is required to be filled, range key is optional , if WithKeyCondition is used in options then range key is not allowed to be set
	indexName string, //pass empty string to query main table
	paginationKey PaginationKey, //from what key to start the query pagination - pass empty struct to start from beginning of the queried table/index
	opts ...QueryOptions,
) (results []T, nextPage PaginationKey, err error) {
	nextPage, err = Query(ctx, client, table, key, indexName, paginationKey, &results, opts...)
	if err != nil {
		return nil, nextPage, err
	}
	return results, nextPage, nil
}

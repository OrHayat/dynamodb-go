package table

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type QueryItemsFromTableQueryOpts struct {
	ExclusiveStartKey map[string]types.AttributeValue
	ConsistentRead    ConsistencyInputType
	ScanOrder         ScanIndexOrderInputType
	FilterBuilder     expression.ConditionBuilder
	ProjectionBuilder *expression.ProjectionBuilder
	Limit             int32
}

func NewQueryItemsInputForTable[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	table Table[KeyType, HashKeyType, RangeKeyType],
	keyMatcher KeyType,
	rangeKeyConditionsBuilder func(rangeKey expression.KeyBuilder) expression.KeyConditionBuilder,
	optsFns ...FuncOption[QueryItemsFromTableQueryOpts],
) (queryInput dynamodb.QueryInput, err error) {

	opts := QueryItemsFromTableQueryOpts{
		ConsistentRead: true,
		ScanOrder:      ScanForward,
	}

	ApplyOptions(&opts, optsFns)

	return newQueryInput(
		table.TableIndexDescriptor,
		keyMatcher,
		table.KeyEncoder,
		rangeKeyConditionsBuilder,
		newQueryInputOpts{
			limit:             opts.Limit,
			scanOrder:         opts.ScanOrder,
			consistentRead:    opts.ConsistentRead,
			filterBuilder:     opts.FilterBuilder,
			projectionBuilder: opts.ProjectionBuilder,
			exclusiveStartKey: opts.ExclusiveStartKey,
		},
	)
}

type QueryItemsFromGSIQueryOpts struct {
	ExclusiveStartKey map[string]types.AttributeValue
	ScanOrder         ScanIndexOrderInputType
	FilterBuilder     expression.ConditionBuilder
	ProjectionBuilder *expression.ProjectionBuilder
	Limit             int32
}

func NewQueryItemsInputForGSI[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	index GlobalSecondaryIndex[KeyType, HashKeyType, RangeKeyType],
	keyMatcher KeyType,
	rangeKeyConditionsBuilder func(rangeKey expression.KeyBuilder) expression.KeyConditionBuilder,
	optsFns ...FuncOption[QueryItemsFromGSIQueryOpts],
) (queryInput dynamodb.QueryInput, err error) {

	opts := QueryItemsFromGSIQueryOpts{
		ScanOrder: ScanForward,
	}

	ApplyOptions(&opts, optsFns)

	return newQueryInput(
		index.TableIndexDescriptor,
		keyMatcher,
		index.KeyEncoder,
		rangeKeyConditionsBuilder,
		newQueryInputOpts{
			limit:             opts.Limit,
			scanOrder:         opts.ScanOrder,
			consistentRead:    false,
			filterBuilder:     opts.FilterBuilder,
			projectionBuilder: opts.ProjectionBuilder,
			exclusiveStartKey: opts.ExclusiveStartKey,
		},
	)
}

type QueryItemsFromLSIQueryOpts struct {
	ExclusiveStartKey map[string]types.AttributeValue
	ConsistentRead    ConsistencyInputType
	ScanOrder         ScanIndexOrderInputType
	FilterBuilder     expression.ConditionBuilder
	ProjectionBuilder *expression.ProjectionBuilder
	Limit             int32
}

func NewQueryItemsInputForLSI[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	index LocalSecondaryIndex[KeyType, HashKeyType, RangeKeyType],
	keyMatcher KeyType,
	rangeKeyConditionsBuilder func(rangeKey expression.KeyBuilder) expression.KeyConditionBuilder,
	optsFns ...FuncOption[QueryItemsFromLSIQueryOpts],
) (queryInput dynamodb.QueryInput, err error) {

	opts := QueryItemsFromLSIQueryOpts{
		ConsistentRead: true,
		ScanOrder:      ScanForward,
	}

	ApplyOptions(&opts, optsFns)

	return newQueryInput(
		index.TableIndexDescriptor,
		keyMatcher,
		index.KeyEncoder,
		rangeKeyConditionsBuilder,
		newQueryInputOpts{
			limit:             opts.Limit,
			scanOrder:         opts.ScanOrder,
			consistentRead:    opts.ConsistentRead,
			filterBuilder:     opts.FilterBuilder,
			projectionBuilder: opts.ProjectionBuilder,
			exclusiveStartKey: opts.ExclusiveStartKey,
		},
	)
}

type newQueryInputOpts struct {
	limit             int32
	scanOrder         ScanIndexOrderInputType
	consistentRead    bool
	filterBuilder     expression.ConditionBuilder
	projectionBuilder *expression.ProjectionBuilder
	exclusiveStartKey map[string]types.AttributeValue
}

func newQueryInput[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](
	desc TableIndexDescriptor,
	keyMatcher KeyType,
	encoder KeyEncoder[KeyType, HashKeyType, RangeKeyType],
	extraRangeKeyConstraints func(rangeKey expression.KeyBuilder) expression.KeyConditionBuilder,
	opts newQueryInputOpts,
) (queryInput dynamodb.QueryInput, err error) {

	hashKeyVal, ok := keyMatcher.HashKey()
	if !ok {
		return queryInput, fmt.Errorf("hash key is required")
	}

	keyCondition := expression.Key(encoder.primaryKeyFieldName).Equal(expression.Value(hashKeyVal))

	var rangeKeyConditions expression.KeyConditionBuilder
	rangeKeyVal, ok := keyMatcher.RangeKey()
	if ok {
		return queryInput, fmt.Errorf("query matcher %#v RangeKey need to be empty to query more then one element but got %v", keyMatcher, rangeKeyVal)
	}

	if extraRangeKeyConstraints != nil {
		rangeKeyConditions = extraRangeKeyConstraints(expression.Key(desc.rangeKeyFieldName))
		if rangeKeyConditions.IsSet() { //if callback return empty dont add it to builder
			keyCondition = keyCondition.And(rangeKeyConditions)
		}
	}

	builder := expression.NewBuilder()
	builder = builder.WithKeyCondition(keyCondition)
	if opts.filterBuilder.IsSet() {
		builder = builder.WithFilter(opts.filterBuilder)
	}

	if opts.projectionBuilder != nil {
		builder = builder.WithProjection(*opts.projectionBuilder)
	}

	expr, err := builder.Build()
	if err != nil {
		return queryInput, err
	}

	queryInput.ExpressionAttributeNames = expr.Names()
	queryInput.ExpressionAttributeValues = expr.Values()
	queryInput.KeyConditionExpression = expr.KeyCondition()
	queryInput.FilterExpression = expr.Filter()
	queryInput.ProjectionExpression = expr.Projection()

	queryInput.TableName = aws.String(desc.TableName)
	if desc.indexName != "" {
		queryInput.IndexName = aws.String(desc.indexName)
	}

	queryInput.ExclusiveStartKey = opts.exclusiveStartKey
	queryInput.ConsistentRead = aws.Bool(opts.consistentRead)
	queryInput.ScanIndexForward = aws.Bool(opts.scanOrder)
	if opts.limit != 0 {
		queryInput.Limit = aws.Int32(opts.limit)
	}

	return queryInput, nil
}

// execute query on given input will return the output and next item token
func ExecuteQuerySinglePage[T any](
	ctx context.Context,
	client dynamodb.QueryAPIClient,
	input *dynamodb.QueryInput,
	optsFns ...DynamoDBFuncOpts,
) (items []T, nextPageToken map[string]types.AttributeValue, err error) {

	output, err := client.Query(ctx, input, optsFns...)
	if err != nil {
		return items, nil, err
	}

	items, err = UnmarshalListOfMaps[T](output.Items)
	if err != nil {
		return items, output.LastEvaluatedKey, err
	}

	return items, output.LastEvaluatedKey, nil
}

type QueryIterator[T any] struct {
	internal         *dynamodb.QueryPaginator
	lastEvaluatedKey map[string]types.AttributeValue
}

func NewQueryItemsIterator[T any](
	client dynamodb.QueryAPIClient,
	input *dynamodb.QueryInput) *QueryIterator[T] {
	return &QueryIterator[T]{
		internal:         dynamodb.NewQueryPaginator(client, input),
		lastEvaluatedKey: input.ExclusiveStartKey,
	}
}

func (i *QueryIterator[T]) HasMorePages() bool {
	return i.internal.HasMorePages()
}

func (i *QueryIterator[T]) GetNextToken() map[string]types.AttributeValue {
	return i.lastEvaluatedKey
}

func (i *QueryIterator[T]) NextPage(ctx context.Context, optsFns ...DynamoDBFuncOpts) (items []T, err error) {

	output, err := i.internal.NextPage(ctx, optsFns...)
	if err != nil {
		return nil, err
	}

	items, err = UnmarshalListOfMaps[T](output.Items)
	if err != nil {
		return items, err
	}

	i.lastEvaluatedKey = output.LastEvaluatedKey

	return items, nil
}

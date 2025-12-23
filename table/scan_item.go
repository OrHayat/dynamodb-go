package table

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/orhayat/dynamodb-go/serializer"
)

type ScanAPIClient interface {
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	GetDecoder() *serializer.Decoder
}

type ScanOptions interface {
	applyScanOption(*ScanConfig)
}

type ScanConfig struct {
}

func prepareScanExpression(scanInput ScanInput) (expr expression.Expression, err error) {
	b := expression.NewBuilder()
	needBuild := false
	if scanInput.FilterExpression.IsSet() {
		needBuild = true
		b = b.WithFilter(scanInput.FilterExpression)
	}
	if scanInput.ProjectionExpression != nil {
		needBuild = true
		b = b.WithProjection(*scanInput.ProjectionExpression)
	}
	if !needBuild {
		return
	}
	return b.Build()
}

func prepareScanRequest(
	table *TableDefinition,
	input ScanInput,
) (*dynamodb.ScanInput, error) {

	startScanFrom, err := input.PaginationKey.resolveExclusiveStartKey(table, input.Index)
	if err != nil {
		return nil, &OperationError{
			operation:   "scan prepare",
			table:       table,
			index:       input.Index,
			internalErr: err,
		}
	}

	expr, err := prepareScanExpression(input)
	if err != nil {
		return nil, &OperationError{
			operation:   "scan prepare",
			table:       table,
			index:       input.Index,
			internalErr: err,
		}
	}

	var limit *int32
	if input.Limit > 0 {
		limit = aws.Int32(input.Limit)
	}
	res := &dynamodb.ScanInput{
		TableName:                 aws.String(table.Name),
		Limit:                     limit,
		ConsistentRead:            aws.Bool(input.ConsistentRead.Bool()),
		ExclusiveStartKey:         startScanFrom,
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		Select:                    "",  //TODO: add way to limit attributes returned - if query index fetch only part of the record or if projection expression is used
		Segment:                   nil, //TODO: add way to parallelize scan - note paginator need to know about segment id
		TotalSegments:             nil, //TODO: add way to parallelize scan
	}

	return res, nil
}

type ScanInput struct {
	Index                string        //pass empty string to scan main table
	PaginationKey        PaginationKey //pass empty struct to start from beginning of table/index
	ConsistentRead       aws.Ternary
	FilterExpression     expression.ConditionBuilder
	ProjectionExpression *expression.ProjectionBuilder
	Limit                int32
}

func Scan(
	ctx context.Context,
	client ScanAPIClient,
	table *TableDefinition,
	input ScanInput,
	out any,
	opts ...ScanOptions,
) (nextPage PaginationKey, err error) {

	decoder := client.GetDecoder()
	if decoder == nil {
		decoder = s_decoder
	}

	scanRequest, err := prepareScanRequest(table, input)
	if err != nil {
		return nextPage, err
	}
	response, err := client.Scan(ctx, scanRequest)
	if err != nil {
		return nextPage, &OperationError{
			operation:   "scan",
			table:       table,
			internalErr: err,
		}
	}
	nextPage = PaginationKey{
		encodedKey: response.LastEvaluatedKey,
	}

	err = serializer.UnmarshalListOfMaps(decoder, response.Items, out)
	if err != nil {
		return nextPage, &OperationError{
			operation:   "scan unmarshal",
			table:       table,
			internalErr: err,
		}
	}

	return nextPage, nil
}

func ScanOf[T any](
	ctx context.Context,
	client ScanAPIClient,
	table *TableDefinition,
	input ScanInput,
	opts ...ScanOptions,
) (page []T, nextPage PaginationKey, err error) {
	nextPage, err = Scan(ctx, client, table, input, &page, opts...)
	return page, nextPage, err
}

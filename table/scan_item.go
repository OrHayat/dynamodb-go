package table

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
	ConsistentRead bool
	Decoder        *serializer.Decoder
	Limit          *int32
}

func prepareScanRequest(
	cfg ScanConfig,
	table *TableDefinition,
	indexName string,
	paginationKey PaginationKey,

) (*dynamodb.ScanInput, error) {

	var startScanFrom map[string]types.AttributeValue
	if paginationKey.useUserKey {
		key, err := table.getKeyForIndex(indexName, paginationKey.userKey)
		if err != nil {
			return nil, &OperationError{
				operation:   "scan prepare",
				table:       table,
				index:       indexName,
				internalErr: err,
			}
		}
		startScanFrom = key
	} else {
		startScanFrom = paginationKey.encodedKey
	}

	res := &dynamodb.ScanInput{
		TableName:                 aws.String(table.Name),
		Limit:                     cfg.Limit,
		ConsistentRead:            aws.Bool(cfg.ConsistentRead),
		ExclusiveStartKey:         startScanFrom,
		ProjectionExpression:      nil, //TODO: add way to support projection expression
		ExpressionAttributeNames:  nil, //needed for filter/projection expression incase of unsupported word in the expression
		ExpressionAttributeValues: nil, //needed for filter/projection expression
		FilterExpression:          nil, //TODO: add way to filter results server side
		Select:                    "",  //TODO: add way to limit attributes returned - if query index fetch only part of the record or if projection expression is used
		Segment:                   nil, //TODO: add way to parallelize scan - note paginator need to know about segment id
		TotalSegments:             nil, //TODO: add way to parallelize scan
	}

	return res, nil
}

func Scan(
	ctx context.Context,
	client ScanAPIClient,
	table *TableDefinition,
	index string, //pass empty string to scan main table
	paginationKey PaginationKey, //pass empty struct to start from beginning of table/index
	out any,
	opts ...ScanOptions,
) (nextPage PaginationKey, err error) {
	cfg := ScanConfig{
		ConsistentRead: false,
	}
	for _, o := range opts {
		o.applyScanOption(&cfg)
	}
	if cfg.Decoder == nil {
		cfg.Decoder = client.GetDecoder()
	}
	if cfg.Decoder == nil {
		cfg.Decoder = s_decoder
	}
	input, err := prepareScanRequest(cfg, table, index, paginationKey)
	if err != nil {
		return nextPage, err
	}
	response, err := client.Scan(ctx, input)
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

	err = serializer.UnmarshalListOfMaps(cfg.Decoder, response.Items, out)
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
	index string, //pass empty string to scan main table
	paginationKey PaginationKey, //pass empty struct to start from beginning of table/index
	opts ...ScanOptions,
) (page []T, nextPage PaginationKey, err error) {
	nextPage, err = Scan(ctx, client, table, index, paginationKey, &page, opts...)
	return page, nextPage, err
}

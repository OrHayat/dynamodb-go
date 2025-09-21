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
	startFrom *Key,
) (*dynamodb.ScanInput, error) {

	var startScanFrom map[string]types.AttributeValue
	if startFrom != nil {
		key, err := table.getKeyForIndex(indexName, *startFrom)
		if err != nil {
			return nil, &OperationError{
				operation:   "scan prepare",
				table:       table,
				internalErr: err,
			}
		}
		startScanFrom = key
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
		Segment:                   nil, //TODO: add way to parallelize scan
		TotalSegments:             nil, //TODO: add way to parallelize scan
	}

	return res, nil
}

func Scan(
	ctx context.Context,
	client ScanAPIClient,
	table *TableDefinition,
	index string, //pass empty string to scan main table
	out any,
	opts ...ScanOptions,
) error {
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
	input, err := prepareScanRequest(cfg, table, index, nil)
	if err != nil {
		return err
	}
	response, err := client.Scan(ctx, input)
	if err != nil {
		return &OperationError{
			operation:   "scan",
			table:       table,
			internalErr: err,
		}
	}
	err = serializer.UnmarshalListOfMaps(cfg.Decoder, response.Items, out)
	if err != nil {
		return &OperationError{
			operation:   "scan unmarshal",
			table:       table,
			internalErr: err,
		}
	}
	return nil
}

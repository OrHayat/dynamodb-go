package table

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/orhayat/dynamodb-go/serializer"
)

type GetItemClient interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error)
	GetDecoder() *serializer.Decoder
}

func prepareGetRequest(
	tableName string,
	key map[string]types.AttributeValue,
) *dynamodb.GetItemInput {
	return &dynamodb.GetItemInput{
		TableName:                &tableName,
		Key:                      key,
		ProjectionExpression:     nil,                              //TODO:add way to generate it
		ExpressionAttributeNames: nil,                              //needed for projection expression incase of unsupported word in the expression useful to not fetch whole record of table across the wire if only part of it needed
		ConsistentRead:           aws.Bool(true),                   //todo:add way to override it this is the safe default for simpler basic api
		ReturnConsumedCapacity:   types.ReturnConsumedCapacityNone, //safe default- usefull for metrics but this package dont help to export metrics
	}
}

func GetItem(
	ctx context.Context,
	client GetItemClient,
	table *TableDefinition,
	pk any,
	sk any,
	out any,
) (err error) {
	encodedKey, err := table.getKey(pk, sk)
	if err != nil {
		return &OperationError{
			operation:   "get item key encoding",
			table:       table,
			pk:          pk,
			sk:          sk,
			internalErr: err,
		}
	}

	request := prepareGetRequest(table.Name, encodedKey)
	res, err := client.GetItem(ctx, request)
	if err != nil {
		return &OperationError{
			operation:   "get item",
			table:       table,
			pk:          pk,
			sk:          sk,
			internalErr: err,
		}
	}

	if len(res.Item) == 0 {
		return &OperationError{
			operation:   "get item",
			table:       table,
			pk:          pk,
			sk:          sk,
			internalErr: ErrItemNotFound,
		}
	}

	err = serializer.UnmarshalMap(client.GetDecoder(), res.Item, out)
	if err != nil {
		return &OperationError{
			operation:   "get item unmarshal",
			table:       table,
			pk:          pk,
			sk:          sk,
			internalErr: err,
		}
	}
	return nil
}

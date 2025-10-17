package table

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/serializer"
)

type TransactionGetItemClient interface {
	TransactGetItems(ctx context.Context, params *dynamodb.TransactGetItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactGetItemsOutput, error)
	GetDecoder() *serializer.Decoder
}
type TransactionGetItemOptions interface {
	applyTransactionGetItemOption(cfg *TransactionGetItemConfig)
}

type TransactionGetItemConfig struct {
	Decoder *serializer.Decoder
}

type TransactionGetRequet struct {
	Table              *TableDefinition
	Key                Key
	Projection         *expression.ProjectionBuilder
	Out                any // result pointer
	AllowItemNotExists bool
}

func prepareGetTransactionRequest(getRequests []TransactionGetRequet) (*dynamodb.TransactGetItemsInput, error) {
	txRequests := make([]types.TransactGetItem, len(getRequests))
	for i, req := range getRequests {
		encodedKey, err := req.Table.getKey(req.Key)
		if err != nil {
			return nil, err
		}
		expr, err := prepareGetExpression(req.Projection)
		if err != nil {
			return nil, err
		}

		txRequests[i] = types.TransactGetItem{
			Get: &types.Get{
				Key:                      encodedKey,
				TableName:                aws.String(req.Table.Name),
				ProjectionExpression:     expr.Projection(),
				ExpressionAttributeNames: expr.Names(),
			},
		}
	}
	request := &dynamodb.TransactGetItemsInput{
		TransactItems:          txRequests,
		ReturnConsumedCapacity: "",
	}

	return request, nil
}

func TransactionGetItem(
	ctx context.Context,
	client TransactionGetItemClient,
	getRequests []TransactionGetRequet,
	opts ...TransactionGetItemOptions,
) (err error) {
	cfg := TransactionGetItemConfig{}

	for _, opt := range opts {
		opt.applyTransactionGetItemOption(&cfg)
	}
	if cfg.Decoder == nil {
		cfg.Decoder = client.GetDecoder()
	}
	if cfg.Decoder == nil {
		cfg.Decoder = s_decoder
	}
	request, err := prepareGetTransactionRequest(getRequests)
	if err != nil {
		return err
	}

	response, err := client.TransactGetItems(ctx, request)
	if err != nil {
		return err
	}

	for i, resp := range response.Responses {
		if resp.Item == nil {
			//if the request allows item not exists - continue - no error getRequests[i].Out will be nil
			if getRequests[i].AllowItemNotExists {
				continue
			}
			return &OperationError{
				operation:   "transaction get item",
				internalErr: ErrItemNotFound,
				table:       getRequests[i].Table,
				pk:          getRequests[i].Key.PK,
				sk:          getRequests[i].Key.SK,
			}
		}

		err = serializer.UnmarshalMap(cfg.Decoder, resp.Item, &getRequests[i].Out)
		if err != nil {
			return &OperationError{
				operation:   "transaction get item unmarshal",
				internalErr: err,
				table:       getRequests[i].Table,
				pk:          getRequests[i].Key.PK,
				sk:          getRequests[i].Key.SK,
			}
		}
	}

	return err
}

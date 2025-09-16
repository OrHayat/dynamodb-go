package table

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/serializer"
)

type BatchPutItemsClient interface {
	BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
	GetEncoder() *serializer.Encoder
}

type BatchWriteItemOptions interface {
	applyBatchWriteItems(cfg *BatchWriteItemConfig)
}

type BatchWriteItemConfig struct {
	Encoder *serializer.Encoder
}

func batchPutRequestPrepareDeleteItems(request WriteRequest) ([]types.WriteRequest, error) {
	deleteRequestBatch := make([]types.WriteRequest, len(request.DeleteItems))
	for i, deleteRequest := range request.DeleteItems {
		encodedKey, err := request.Table.getKey(deleteRequest.PK, deleteRequest.SK)
		if err != nil {
			//TODO improve error!!!
			return nil, err
		}
		deleteRequestBatch[i].DeleteRequest.Key = encodedKey
	}
	return deleteRequestBatch, nil
}

func batchPutRequestPreparePutItems(
	request WriteRequest,
	encoder *serializer.Encoder,
) ([]types.WriteRequest, error) {

	putRequestBatch := make([]types.WriteRequest, len(request.DeleteItems))
	for i, putRequest := range request.PutRequests {
		encodedItem, err := serializer.MarshalMap(encoder, putRequest.Item)
		if err != nil {
			return nil, err
		}
		pk, sk, err := request.Table.ExtractKeys(encodedItem)
		if err != nil {
			//TODO improve error!!! ,use pk and sk
			return nil, err
		}
		_ = pk
		_ = sk
		putRequestBatch[i].PutRequest.Item = encodedItem
	}
	return putRequestBatch, nil
}

func prepareBatchWriteItemsRequest(
	requests []WriteRequest,
	encoder *serializer.Encoder,
) (*dynamodb.BatchWriteItemInput, error) {
	preparedRequests := map[string][]types.WriteRequest{}
	for i, request := range requests {
		tableName := request.Table.Name

		deleteRequests, err := batchPutRequestPrepareDeleteItems(request)
		if err != nil {
			return nil, fmt.Errorf("failed to encode batch write request %d for table %s:%w", i, tableName, err)
		}
		preparedRequests[tableName] = append(preparedRequests[tableName], deleteRequests...)

		putRequests, err := batchPutRequestPreparePutItems(request, encoder)
		if err != nil {
			return nil, fmt.Errorf("failed to encode batch write request %d for table %s:%w", i, tableName, err)
		}
		preparedRequests[tableName] = append(preparedRequests[tableName], putRequests...)
	}

	res := &dynamodb.BatchWriteItemInput{
		RequestItems:                preparedRequests,
		ReturnConsumedCapacity:      "",
		ReturnItemCollectionMetrics: "",
	}
	return res, nil
}

type DeleteRequest struct {
	PK any
	SK any
}
type PutRequest struct {
	Item any
}
type WriteRequest struct {
	Table       *TableDefinition
	DeleteItems []DeleteRequest
	PutRequests []PutRequest
}

func BatchWriteItems(
	ctx context.Context,
	client BatchPutItemsClient,
	requests []WriteRequest,
	opts ...BatchWriteItemOptions,
) (err error) {
	cfg := BatchWriteItemConfig{}
	for _, opt := range opts {
		opt.applyBatchWriteItems(&cfg)
	}
	if cfg.Encoder == nil {
		cfg.Encoder = client.GetEncoder()
	}
	if cfg.Encoder == nil {
		cfg.Encoder = s_encoder
	}

	request, err := prepareBatchWriteItemsRequest(requests, client.GetEncoder())
	if err != nil {
		return err
	}
	//TODO: handle unprocessed items
	//return in the case of error:
	//processed items
	//unprocessed items
	_, err = client.BatchWriteItem(ctx, request)
	if err != nil {
		return err
	}
	return nil
}

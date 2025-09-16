package table

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

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
		encodedKey, err := request.Table.getKey(deleteRequest.Key)
		if err != nil {
			return nil, &OperationError{
				operation:   "encode put request",
				internalErr: fmt.Errorf("failed to encode put request %d in the batch :%w", i, err),
				table:       request.Table,
				pk:          deleteRequest.PK,
				sk:          deleteRequest.SK,
			}
		}
		deleteRequestBatch[i].DeleteRequest = &types.DeleteRequest{
			Key: encodedKey,
		}
	}
	return deleteRequestBatch, nil
}

func batchPutRequestPreparePutItems(
	request WriteRequest,
	encoder *serializer.Encoder,
) ([]types.WriteRequest, error) {
	putRequestBatch := make([]types.WriteRequest, len(request.PutRequests))
	for i, putRequest := range request.PutRequests {
		encodedItem, err := serializer.MarshalMap(encoder, putRequest.Item)
		if err != nil {
			return nil, &OperationError{
				operation:   "marshal put request",
				internalErr: err,
				table:       request.Table,
				pk:          nil,
				sk:          nil,
			}
		}
		pk, sk, err := request.Table.ExtractKeys(encodedItem)
		if err != nil {
			return nil, &OperationError{
				operation:   "put request keys validation",
				internalErr: fmt.Errorf("failed to encode delete request %d in the batch :%w", i, err),
				table:       request.Table,
				pk:          request.Table.encodedKeyToVal(pk),
				sk:          request.Table.encodedKeyToVal(sk),
			}
		}
		putRequestBatch[i].PutRequest = &types.PutRequest{
			Item: encodedItem,
		}
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
			return nil, fmt.Errorf("failed to encode request number %d:%w", i, err)
		}
		preparedRequests[tableName] = append(preparedRequests[tableName], deleteRequests...)

		putRequests, err := batchPutRequestPreparePutItems(request, encoder)
		if err != nil {
			return nil, fmt.Errorf("failed to encode batch write request number %d for table %s:%w", i, tableName, err)
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
	Key
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

	request, err := prepareBatchWriteItemsRequest(requests, cfg.Encoder)
	if err != nil {
		return err
	}

	attempt := 0
	for {
		out, err := client.BatchWriteItem(ctx, request)
		if err != nil {
			return err
		}
		request.RequestItems = out.UnprocessedItems
		if len(out.UnprocessedItems) == 0 {
			break
		}
		delay := backoffDelay(attempt, 15, time.Millisecond*300)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// continue and do the next attempt
		}
		attempt++
	}
	return nil
}

func backoffDelay(attempt int, maxAttempts int, maxBackoff time.Duration) time.Duration {
	if attempt > maxAttempts {
		return maxBackoff
	}

	b := rand.Float64()

	// [0.0, 1.0) * 2 ^ attempts
	ri := int64(1 << uint64(attempt))
	delaySeconds := b * float64(ri)

	return time.Second * time.Duration(delaySeconds)
}

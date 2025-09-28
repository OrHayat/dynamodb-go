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
}
type TransactionGetRequet struct {
	Table              *TableDefinition
	Key                Key
	Projection         *expression.ProjectionBuilder
	Out                any //result pointer
	AllowItemNotExists bool
}

func prepareGetTransactionRequest(getRequests []TransactionGetRequet) (*dynamodb.TransactGetItemsInput, error) {
	txRequests := make([]types.TransactGetItem, len(getRequests))
	for i, req := range getRequests {
		encodedKey, err := req.Table.getKey(req.Key)
		if err != nil {
			return nil, err
		}
		var proejction *string
		var names map[string]string
		if req.Projection != nil {
			b := expression.NewBuilder().WithProjection(*req.Projection)
			expr, err := b.Build()
			if err != nil {
				return nil, err
			}
			proejction = expr.Projection()
			names = expr.Names()
		}

		txRequests[i] = types.TransactGetItem{
			Get: &types.Get{
				Key:                      encodedKey,
				TableName:                aws.String(req.Table.Name),
				ProjectionExpression:     proejction,
				ExpressionAttributeNames: names,
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
) (err error) {

	request, err := prepareGetTransactionRequest(getRequests)
	if err != nil {
		return err
	}

	response, err := client.TransactGetItems(ctx, request)
	if err != nil {
		return err
	}

	for i, resp := range response.Responses {
		//item not found
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

		err = serializer.UnmarshalMap(nil, resp.Item, &getRequests[i].Out)
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

// func TransactGetItems(ctx context.Context) (err error) {
// 	c := dynamodb.Client{}
// 	request, er := prepareGetTransactionRequest()
// 	if er != nil {
// 		return
// 	}
// 	out, err := c.TransactGetItems(ctx, request)
// 	if err != nil {
// 		// Handle error
// 		return
// 	}

// 	// outputs := out.Responses[0].
// 	// Use output
// }

// func prepareWriteTransactionRequest() (*dynamodb.TransactWriteItemsInput, error) {
// 	request := &dynamodb.TransactWriteItemsInput{
// 		TransactItems: []types.TransactWriteItem{
// 			{
// 				Put: &types.Put{
// 					Item:                      nil,
// 					TableName:                 nil,
// 					ConditionExpression:       nil,
// 					ExpressionAttributeNames:  nil,
// 					ExpressionAttributeValues: nil,
// 				},
// 				Delete: &types.Delete{
// 					Key:                       nil,
// 					TableName:                 nil,
// 					ConditionExpression:       nil,
// 					ExpressionAttributeNames:  nil,
// 					ExpressionAttributeValues: nil,
// 				},
// 				Update: &types.Update{
// 					Key:                       nil,
// 					TableName:                 nil,
// 					UpdateExpression:          nil,
// 					ConditionExpression:       nil,
// 					ExpressionAttributeNames:  nil,
// 					ExpressionAttributeValues: nil,
// 				},
// 				ConditionCheck: &types.ConditionCheck{
// 					Key:                       nil,
// 					TableName:                 nil,
// 					ConditionExpression:       nil,
// 					ExpressionAttributeNames:  nil,
// 					ExpressionAttributeValues: nil,
// 				},
// 			},
// 		},
// 	}
// 	return request, nil
// }
// func TransactWriteItems(ctx context.Context) (err error) {
// 	c := dynamodb.Client{}
// 	request, er := prepareGetTransactionRequest()
// 	if er != nil {
// 		return
// 	}
// 	out, err := c.TransactWriteItems(ctx, request)
// 	if err != nil {
// 		// Handle error
// 		return
// 	}

// 	return nil
// 	// outputs := out.Responses[0].
// 	// Use output
// }

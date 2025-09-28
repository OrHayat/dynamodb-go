package table

// import (
// 	"context"

// 	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
// 	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
// )

// func prepareGetTransactionRequest() (*dynamodb.TransactGetItemsInput, error) {
// 	request := &dynamodb.TransactGetItemsInput{
// 		TransactItems: []types.TransactGetItem{
// 			{
// 				Get: &types.Get{
// 					Key:                      nil,
// 					TableName:                nil,
// 					ProjectionExpression:     nil,
// 					ExpressionAttributeNames: nil,
// 				},
// 			},
// 		},
// 	}
// 	return request, nil
// }

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

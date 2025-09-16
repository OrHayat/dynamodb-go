package table

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DeleteTableClient interface {
	DeleteTable(ctx context.Context, params *dynamodb.DeleteTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DeleteTableOutput, error)
}

func DeleteTable(
	ctx context.Context,
	client DeleteTableClient,
	table *TableDefinition,
) (err error) {
	input := &dynamodb.DeleteTableInput{
		TableName: &table.Name,
	}
	_, err = client.DeleteTable(ctx, input)
	if err != nil {
		if _, ok := ErrorAs[*types.ResourceNotFoundException](err); ok {
			// Table not found, no action needed
			return nil
		}
		return &OperationError{
			operation:   "delete table",
			table:       table,
			internalErr: err,
		}
	}
	return nil
}

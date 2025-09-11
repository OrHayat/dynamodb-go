package table

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type CreateTableClient interface {
	CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error)
}

func prepareCreateTableRequest(table *TableDefinition) *dynamodb.CreateTableInput {
	attributes := []types.AttributeDefinition{
		{
			AttributeName: &table.PrimaryKey.Name,
			AttributeType: table.PrimaryKey.Type,
		},
	}
	if table.RangeKey.Name != "" {
		attributes = append(attributes, types.AttributeDefinition{
			AttributeName: &table.RangeKey.Name,
			AttributeType: table.RangeKey.Type,
		})
	}
	keySchema := []types.KeySchemaElement{
		{
			AttributeName: &table.PrimaryKey.Name,
			KeyType:       types.KeyTypeHash,
		},
	}
	if table.RangeKey.Name != "" {
		keySchema = append(keySchema, types.KeySchemaElement{
			AttributeName: &table.RangeKey.Name,
			KeyType:       types.KeyTypeRange,
		})
	}
	input := &dynamodb.CreateTableInput{
		TableName:            &table.Name,
		AttributeDefinitions: attributes,
		BillingMode:          types.BillingModePayPerRequest,
		KeySchema:            keySchema,
	}
	return input
}

func CreateTable(
	ctx context.Context,
	client CreateTableClient,
	table *TableDefinition,
) (err error) {

	input := prepareCreateTableRequest(table)
	_, err = client.CreateTable(ctx, input)
	if err != nil {
		return &OperationError{
			operation:   "create table",
			table:       table,
			internalErr: err,
		}
	}
	return nil
}

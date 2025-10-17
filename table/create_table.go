package table

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type CreateTableClient interface {
	CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error)
}

func prepareCreateTableRequest(table *TableDefinition) (*dynamodb.CreateTableInput, error) {
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
	var billingMode types.BillingMode = types.BillingModePayPerRequest
	var requestGSI []types.GlobalSecondaryIndex
	for _, gsi := range table.GSI {
		curr := types.GlobalSecondaryIndex{
			IndexName: aws.String(gsi.IndexName),
			KeySchema: []types.KeySchemaElement{},
		}
		curr.KeySchema = append(curr.KeySchema, types.KeySchemaElement{
			AttributeName: aws.String(gsi.PrimaryKey.Name),
			KeyType:       types.KeyTypeHash,
		})
		if gsi.RangeKey.Name != "" {
			curr.KeySchema = append(curr.KeySchema, types.KeySchemaElement{
				AttributeName: aws.String(gsi.RangeKey.Name),
				KeyType:       types.KeyTypeRange,
			})
		}
		requestGSI = append(requestGSI, curr)
	}

	var requestLSI []types.LocalSecondaryIndex
	for _, lsi := range table.LSI {
		curr := types.LocalSecondaryIndex{
			IndexName: aws.String(lsi.IndexName),
			KeySchema: []types.KeySchemaElement{},
		}
		if lsi.RangeKey.Name == "" {
			return nil, fmt.Errorf("LSI must have range key")
		}
		curr.KeySchema = append(curr.KeySchema, types.KeySchemaElement{
			AttributeName: aws.String(lsi.RangeKey.Name),
			KeyType:       types.KeyTypeRange,
		})
		requestLSI = append(requestLSI, curr)
	}

	input := &dynamodb.CreateTableInput{
		TableName:              &table.Name,
		AttributeDefinitions:   attributes,
		BillingMode:            billingMode,
		KeySchema:              keySchema,
		GlobalSecondaryIndexes: requestGSI,
		LocalSecondaryIndexes:  requestLSI,
	}

	return input, nil
}

func CreateTable(
	ctx context.Context,
	client CreateTableClient,
	table *TableDefinition,
) (err error) {

	input, err := prepareCreateTableRequest(table)
	if err != nil {
		return err
	}
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

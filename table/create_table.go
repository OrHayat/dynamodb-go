package table

import (
	"context"
	"fmt"

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
	var billingMode types.BillingMode
	if table.BillingMode == "" {
		billingMode = types.BillingModePayPerRequest
		table.BillingMode = billingMode
	}

	if table.BillingMode != types.BillingModeProvisioned && table.ProvisionedThroughput != nil {
		return nil, fmt.Errorf("ProvisionedThroughput must be nil when BillingMode is not PROVISIONED")
	}
	if table.BillingMode != types.BillingModePayPerRequest && table.OnDemandThroughput != nil {
		return nil, fmt.Errorf("OnDemandThroughput must be nil when BillingMode is not PAY_PER_REQUEST")
	}
	input := &dynamodb.CreateTableInput{
		TableName:              &table.Name,
		AttributeDefinitions:   attributes,
		BillingMode:            billingMode,
		ProvisionedThroughput:  table.ProvisionedThroughput,
		OnDemandThroughput:     table.OnDemandThroughput,
		KeySchema:              keySchema,
		GlobalSecondaryIndexes: nil,
		LocalSecondaryIndexes:  nil,
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

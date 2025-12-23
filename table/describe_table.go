package table

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DescribeTableClient interface {
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

func DescribeTable(
	ctx context.Context,
	client DescribeTableClient,
	tableName string,
) (tableSchema TableDefinition, tableDescription *types.TableDescription, err error) {

	response, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return TableDefinition{}, nil, err
	}
	tableDescription = response.Table

	// Handle billing info (may be nil for older tables or certain configurations)
	if tableDescription.BillingModeSummary != nil {
		tableSchema.Billing.BillingMode = tableDescription.BillingModeSummary.BillingMode
		switch tableDescription.BillingModeSummary.BillingMode {
		case types.BillingModeProvisioned:
			if desc := tableDescription.ProvisionedThroughput; desc != nil {
				tableSchema.Billing.ProvisionedThroughput = &types.ProvisionedThroughput{
					ReadCapacityUnits:  desc.ReadCapacityUnits,
					WriteCapacityUnits: desc.WriteCapacityUnits,
				}
			}
		case types.BillingModePayPerRequest:
			if desc := tableDescription.OnDemandThroughput; desc != nil {
				tableSchema.Billing.OnDemandThroughput = &types.OnDemandThroughput{
					MaxReadRequestUnits:  desc.MaxReadRequestUnits,
					MaxWriteRequestUnits: desc.MaxWriteRequestUnits,
				}
			}
		}
	}
	tableSchema.Name = aws.ToString(tableDescription.TableName)

	// Build map of attribute name -> scalar type from AttributeDefinitions
	attrTypes := make(map[string]types.ScalarAttributeType)
	for _, attr := range tableDescription.AttributeDefinitions {
		attrTypes[aws.ToString(attr.AttributeName)] = attr.AttributeType
	}

	for _, key := range tableDescription.KeySchema {
		attrName := aws.ToString(key.AttributeName)
		switch key.KeyType {
		case types.KeyTypeHash:
			tableSchema.PrimaryKey = AttributeDefinition{
				Name: attrName,
				Type: attrTypes[attrName],
			}
		case types.KeyTypeRange:
			tableSchema.RangeKey = AttributeDefinition{
				Name: attrName,
				Type: attrTypes[attrName],
			}
		}
	}
	for _, gsi := range tableDescription.GlobalSecondaryIndexes {
		curr := GlobalSecondaryIndex{
			IndexName: aws.ToString(gsi.IndexName),
		}
		for _, key := range gsi.KeySchema {
			attrName := aws.ToString(key.AttributeName)
			switch key.KeyType {
			case types.KeyTypeHash:
				curr.PrimaryKey = AttributeDefinition{
					Name: attrName,
					Type: attrTypes[attrName],
				}
			case types.KeyTypeRange:
				curr.RangeKey = AttributeDefinition{
					Name: attrName,
					Type: attrTypes[attrName],
				}
			}
		}
		tableSchema.GSI = append(tableSchema.GSI, curr)
	}

	for _, lsi := range tableDescription.LocalSecondaryIndexes {
		curr := LocalSecondaryIndex{
			IndexName: aws.ToString(lsi.IndexName),
		}
		for _, key := range lsi.KeySchema {
			attrName := aws.ToString(key.AttributeName)
			switch key.KeyType {
			case types.KeyTypeHash:
				// Skip - LSI inherits partition key from table
				continue
			case types.KeyTypeRange:
				curr.RangeKey = AttributeDefinition{
					Name: attrName,
					Type: attrTypes[attrName],
				}
			}
		}
		tableSchema.LSI = append(tableSchema.LSI, curr)
	}

	return tableSchema, tableDescription, nil
}

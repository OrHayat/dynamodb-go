package table

import (
	"context"
	"fmt"

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
	tableSchema.Billing.BillingMode = tableDescription.BillingModeSummary.BillingMode
	switch tableDescription.BillingModeSummary.BillingMode {
	case types.BillingModeProvisioned:
		desc := tableDescription.ProvisionedThroughput
		tableSchema.Billing.ProvisionedThroughput = &types.ProvisionedThroughput{
			ReadCapacityUnits:  desc.ReadCapacityUnits,
			WriteCapacityUnits: desc.WriteCapacityUnits,
		}
	case types.BillingModePayPerRequest:
		desc := tableDescription.OnDemandThroughput
		tableSchema.Billing.OnDemandThroughput = &types.OnDemandThroughput{
			MaxReadRequestUnits:  desc.MaxReadRequestUnits,
			MaxWriteRequestUnits: desc.MaxWriteRequestUnits,
		}
	}
	tableSchema.Name = aws.ToString(tableDescription.TableName)

	for _, key := range tableDescription.KeySchema {
		// key.KeyType == types.KeyTypeHash
		switch key.KeyType {
		case types.KeyTypeHash:
			tableSchema.PrimaryKey = AttributeDefinition{
				Name: aws.ToString(key.AttributeName),
				Type: types.ScalarAttributeType(key.KeyType),
			}
		case types.KeyTypeRange:
			tableSchema.RangeKey = AttributeDefinition{
				Name: aws.ToString(key.AttributeName),
				Type: types.ScalarAttributeType(key.KeyType),
			}
		}
	}
	for _, gsi := range tableDescription.GlobalSecondaryIndexes {
		curr := GlobalSecondaryIndex{
			IndexName: aws.ToString(gsi.IndexName),
		}
		for _, key := range gsi.KeySchema {
			switch key.KeyType {
			case types.KeyTypeHash:
				curr.PrimaryKey = AttributeDefinition{
					Name: aws.ToString(key.AttributeName),
					Type: types.ScalarAttributeType(key.KeyType),
				}
			case types.KeyTypeRange:
				curr.RangeKey = AttributeDefinition{
					Name: aws.ToString(key.AttributeName),
					Type: types.ScalarAttributeType(key.KeyType),
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
			switch key.KeyType {
			case types.KeyTypeHash:
				return TableDefinition{}, tableDescription, fmt.Errorf("unepected output from describe table - lsi %s has primary key", aws.ToString(lsi.IndexName))
			case types.KeyTypeRange:
				curr.RangeKey = AttributeDefinition{
					Name: aws.ToString(key.AttributeName),
					Type: types.ScalarAttributeType(key.KeyType),
				}
			}
		}
		// lsi.Projection
		//arn
		// _ = lsi.IndexArn
		// _ = lsi.Projection
		tableSchema.LSI = append(tableSchema.LSI, curr)
	}

	return tableSchema, tableDescription, nil
}

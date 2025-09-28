package table

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/orhayat/dynamodb-go/serializer"
)

type UpsertItemOptions interface {
	applyUpsertItemOption(*UpsertItemConfig)
}

type UpsertItemConfig struct {
	Encoder          *serializer.Encoder
	ConditionalCheck expression.ConditionBuilder
}

// put item in given table
// if item exists, replace it
// this function also allows passing custom conditional check from user side in UpsertItemOptions
// that allows dynamoDB reject replacing of existing item(for example replacing existing item with item version check)
func UpsertItem(
	ctx context.Context,
	client PutItemClient,
	table *TableDefinition,
	item any,
	opts ...UpsertItemOptions,
) (err error) {

	cfg := &UpsertItemConfig{
		Encoder: nil,
	}
	for _, opt := range opts {
		opt.applyUpsertItemOption(cfg)
	}
	if cfg.Encoder == nil {
		cfg.Encoder = client.GetEncoder()
	}
	if cfg.Encoder == nil {
		cfg.Encoder = s_encoder
	}

	if cfg.ConditionalCheck.IsSet() {
		//item doesnt exists
		conditionExpression := expression.AttributeNotExists(expression.Name(table.PrimaryKey.Name))
		if table.RangeKey.Name != "" {
			conditionExpression = conditionExpression.And(expression.AttributeNotExists(expression.Name(table.RangeKey.Name)))
		}
		//or user condition on existing item
		cfg.ConditionalCheck = conditionExpression.Or(cfg.ConditionalCheck)
	}
	return putOrUpsertItem(ctx, client, table, item, cfg.Encoder, cfg.ConditionalCheck, true)
}

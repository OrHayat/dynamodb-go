package table

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/orhayat/dynamodb-go/serializer"
)

type UpdateItemsClient interface {
	Update(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	GetEncoder() *serializer.Encoder
}

type UpdateOptions interface {
	applyUpdateOption(*UpdateConfig)
}
type UpdateConfig struct {
	Encoder           *serializer.Encoder
	PreventsOverwrite bool                        //if true will prevent overwriting existing attributes
	ConditionalCheck  expression.ConditionBuilder //additional condition expression to add to the update request incase user wants to add custom condition on existing item
}

var ErrPrimaryOrSortKeyUpdate = errors.New("cannot update primary key or sort key of the table; use a delete + write transaction instead")

func prepareUpdateRequest(table *TableDefinition, key Key, item any, cfg UpdateConfig, checkItemExists bool) (*dynamodb.UpdateItemInput, error) {

	encodedKey, err := table.getKey(key)
	if err != nil {
		return nil, err
	}

	updateParams, err := serializer.MarshalMap(cfg.Encoder, item)
	if err != nil {
		return nil, err
	}

	if _, ok := updateParams[table.PrimaryKey.Name]; ok {
		return nil, &OperationError{
			operation:   "prepare update item",
			internalErr: ErrPrimaryOrSortKeyUpdate,
			table:       table,
		}
	}

	if table.RangeKey.Name != "" {
		if _, ok := updateParams[table.RangeKey.Name]; ok {
			return nil, ErrPrimaryOrSortKeyUpdate
		}
	}
	if len(updateParams) == 0 {
		return nil, errors.New("no fields to update")
	}
	b := expression.NewBuilder()
	update := expression.UpdateBuilder{}
	for k, v := range updateParams {
		val := expression.Value(v)
		name := expression.Name(k)
		if cfg.PreventsOverwrite {
			update = update.Set(name, expression.IfNotExists(name, val))
		} else {
			update = update.Set(name, val)
		}
	}
	b = b.WithUpdate(update)

	if checkItemExists { //item must exists, so add condition to check item exists
		condition := ensureKeyExists(table)
		if cfg.ConditionalCheck.IsSet() { //there is extra condition on the item .. add it
			condition = condition.And(cfg.ConditionalCheck)
		}
		b = b.WithCondition(condition)
	} else { //in this case item can be in db (update) or not (create)
		//assume that the cfg.ConditionalCheck is conditonal check that is on the item if it exists
		//and add case that will not fail the request if the itme doesnt exists
		if cfg.ConditionalCheck.IsSet() {
			condition := ensureKeyNotExists(table)
			condition = condition.Or(cfg.ConditionalCheck)
			b = b.WithCondition(condition)
		}
	}
	expr, err := b.Build()
	if err != nil {
		return nil, err
	}
	request := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(table.Name),
		Key:                       encodedKey,
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              "", //TODO: support return values option
	}
	return request, nil
}

type ItemUpdate struct {
	UpdateFields any //either struct or map[string]any
}

func createOrUpdateItemInternal(ctx context.Context, client UpdateItemsClient, table *TableDefinition, key Key, update ItemUpdate, checkItemExists bool, opts ...UpdateOptions) (err error) {
	cfg := UpdateConfig{}
	for _, opt := range opts {
		opt.applyUpdateOption(&cfg)
	}
	if cfg.Encoder == nil {
		cfg.Encoder = client.GetEncoder()
	}
	if cfg.Encoder == nil {
		cfg.Encoder = s_encoder
	}
	request, err := prepareUpdateRequest(table, key, update.UpdateFields, cfg, checkItemExists)
	if err != nil {
		return &OperationError{
			operation:   "update item prepare request",
			table:       table,
			pk:          key.PK,
			sk:          key.SK,
			internalErr: err,
		}
	}
	_, err = client.Update(ctx, request)
	if err != nil {
		return &OperationError{
			operation:   "update item",
			table:       table,
			pk:          key.PK,
			sk:          key.SK,
			internalErr: err,
		}
	}
	return nil
}
func UpdateItem(ctx context.Context, client UpdateItemsClient, table *TableDefinition, key Key, update ItemUpdate, opts ...UpdateOptions) (err error) {
	return createOrUpdateItemInternal(ctx, client, table, key, update, true, opts...)
}

func UpsertItem(ctx context.Context, client UpdateItemsClient, table *TableDefinition, key Key, update ItemUpdate, opts ...UpdateOptions) (err error) {
	return createOrUpdateItemInternal(ctx, client, table, key, update, false, opts...)
}

func ensureKeyExists(table *TableDefinition) expression.ConditionBuilder {
	condition := expression.AttributeExists(expression.Name(table.PrimaryKey.Name))
	if table.RangeKey.Name != "" {
		condition = condition.And(expression.AttributeExists(expression.Name(table.RangeKey.Name)))
	}
	return condition
}

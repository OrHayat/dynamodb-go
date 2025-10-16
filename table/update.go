package table

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
	Encoder *serializer.Encoder
}

var ErrPrimaryOrSortKeyUpdate = errors.New("cannot update primary key or sort key of the table; use a delete + write transaction instead")

func prepareUpdateExpression(
	table *TableDefinition,
	encoder *serializer.Encoder,
	item any,
	preverntsOverrite bool,
	upsert bool,
	conditionalCheck expression.ConditionBuilder,
) (expression.Expression, error) {

	updateParams, err := serializer.MarshalMap(encoder, item)
	if err != nil {
		return expression.Expression{}, err
	}

	if _, ok := updateParams[table.PrimaryKey.Name]; ok {
		return expression.Expression{}, &OperationError{
			operation:   "prepare update item",
			internalErr: ErrPrimaryOrSortKeyUpdate,
			table:       table,
		}
	}

	if table.RangeKey.Name != "" {
		if _, ok := updateParams[table.RangeKey.Name]; ok {
			return expression.Expression{}, ErrPrimaryOrSortKeyUpdate
		}
	}
	if len(updateParams) == 0 {
		return expression.Expression{}, errors.New("no fields to update")
	}

	b := expression.NewBuilder()
	update := expression.UpdateBuilder{}
	for k, v := range updateParams {
		val := expression.Value(v)
		name := expression.Name(k)
		if preverntsOverrite {
			update = update.Set(name, expression.IfNotExists(name, val))
		} else {
			update = update.Set(name, val)
		}
	}
	b = b.WithUpdate(update)

	if upsert {
		//in this case item can be in db (update) or not (create)
		//assume that the cfg.ConditionalCheck is conditonal check that is on the item if it exists
		//and add case that will not fail the request if the itme doesnt exists
		if conditionalCheck.IsSet() {
			condition := ensureKeyNotExists(table)
			condition = condition.Or(conditionalCheck)
			b = b.WithCondition(condition)
		}
	} else {
		//item must exists, so add condition to check item exists
		condition := ensureKeyExists(table)
		if conditionalCheck.IsSet() { //there is extra condition on the item .. add it
			condition = condition.And(conditionalCheck)
		}
		b = b.WithCondition(condition)
	}

	return b.Build()
}

func prepareUpdateRequest(
	table *TableDefinition,
	key Key,
	update ItemUpdate,
	cfg UpdateConfig,
	checkItemExists bool,
) (*dynamodb.UpdateItemInput, error) {

	encodedKey, err := table.getKey(key)
	if err != nil {
		return nil, err
	}

	expr, err := prepareUpdateExpression(
		table,
		cfg.Encoder,
		update.UpdateFields,
		update.PreventsOverwrite.Bool(),
		checkItemExists,
		update.ConditionalCheck,
	)
	if err != nil {
		return nil, err
	}

	request := &dynamodb.UpdateItemInput{
		TableName:                           aws.String(table.Name),
		Key:                                 encodedKey,
		UpdateExpression:                    expr.Update(),
		ConditionExpression:                 expr.Condition(),
		ExpressionAttributeNames:            expr.Names(),
		ExpressionAttributeValues:           expr.Values(),
		ReturnValues:                        "", //TODO: support return values option
		ReturnValuesOnConditionCheckFailure: types.ReturnValuesOnConditionCheckFailureAllOld,
	}
	return request, nil
}

type ItemUpdate struct {
	//must be either struct or map[string]any
	UpdateFields any
	//if this is true - update will do upsert operation
	//
	// if its false, it will add check for item exsistence to not allow insertion in update operation.
	//
	// defaults to false
	Upsert aws.Ternary
	//if true will prevent overwriting existing attributes
	//
	//  defaults to false
	PreventsOverwrite aws.Ternary
	//additional condition expression to add to the update request to check the updated item.
	//
	//if this option  is used updated check for item exsistence is added in the query
	ConditionalCheck expression.ConditionBuilder
}

func createOrUpdateItemInternal(
	ctx context.Context,
	client UpdateItemsClient,
	table *TableDefinition,
	key Key,
	update ItemUpdate,
	checkItemExists bool,
	opts ...UpdateOptions,
) (err error) {
	if update.PreventsOverwrite == aws.UnknownTernary {
		update.PreventsOverwrite = aws.FalseTernary
	}
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
	request, err := prepareUpdateRequest(table, key, update, cfg, checkItemExists)
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

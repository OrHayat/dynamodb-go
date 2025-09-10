package table

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/orhayat/dynamodb-go/serializer"
)

type GetItemClient interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error)
	GetDecoder() *serializer.Decoder
}

func prepareGetRequest(
	tableName string,
	key map[string]types.AttributeValue,
) *dynamodb.GetItemInput {
	return &dynamodb.GetItemInput{
		TableName:                &tableName,
		Key:                      key,
		ProjectionExpression:     nil,                              //TODO:add way to generate it
		ExpressionAttributeNames: nil,                              //needed for projection expression incase of unsupported word in the expression useful to not fetch whole record of table across the wire if only part of it needed
		ConsistentRead:           aws.Bool(true),                   //todo:add way to override it this is the safe default for simpler basic api
		ReturnConsumedCapacity:   types.ReturnConsumedCapacityNone, //safe default- usefull for metrics but this package dont help to export metrics
	}
}

type GetItemError struct {
	Err   error
	Op    string
	PK    any
	SK    any
	table *TableDefinition
}

func (e GetItemError) Unwrap() error {
	return e.Err
}

func (e GetItemError) Error() string {
	var b strings.Builder
	b.WriteString("get item failed")
	// default prefix
	if e.Op != "" {
		b.WriteString(" (" + e.Op + ")")
	} else {
		// Op empty → missing item case, no prefix
	}

	// table name
	if e.table != nil {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		b.WriteString("for table ")
		b.WriteString(e.table.Name)
	}

	// PK / SK individually
	if e.PK != nil || e.SK != nil {
		b.WriteString("{")
		added := false
		if e.PK != nil {
			var pkName string
			if e.table != nil {
				pkName = e.table.PrimaryKey.Name
			} else {
				pkName = "PK"
			}
			fmt.Fprintf(&b, "%s:%v", pkName, e.PK)
			added = true
		}
		if e.SK != nil {
			if added {
				b.WriteString(", ")
			}
			var skName string
			if e.table != nil {
				skName = e.table.RangeKey.Name
			} else {
				skName = "SK"
			}
			fmt.Fprintf(&b, "%s:%v", skName, e.SK)
		}
		b.WriteString("}")
	}

	// underlying error
	if e.Err != nil {
		b.WriteString(e.Err.Error())
	}

	return b.String()
}

func GetItem[T any](
	ctx context.Context,
	client GetItemClient,
	table *TableDefinition,
	pk any,
	sk any,
	out any,
) (err error) {
	encodedKey, err := table.getKey(pk, sk)
	if err != nil {
		return GetItemError{Err: err}
	}

	request := prepareGetRequest(table.Name, encodedKey)
	res, err := client.GetItem(ctx, request)
	if err != nil {
		return GetItemError{
			Op:    "request",
			table: table,
			PK:    pk,
			SK:    sk,
			Err:   err,
		}
	}

	if len(res.Item) == 0 {
		return GetItemError{
			Op:    "",
			table: table,
			PK:    pk,
			SK:    sk,
			Err:   ErrItemNotFound,
		}
	}

	err = serializer.UnmarshalMap(client.GetDecoder(), res.Item, out)
	if err != nil {
		return GetItemError{
			Op:    "unmarshal",
			table: table,
			PK:    pk,
			SK:    sk,
			Err:   err,
		}
	}
	return nil
}

// func GetItemeX() {

// }

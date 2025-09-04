package table

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// alias to bool for readability
type ConsistencyInputType = bool

const (
	ConsistentRead    ConsistencyInputType = true
	NotConsistentRead ConsistencyInputType = false
)

// alias to bool for readability
type ScanIndexOrderInputType = bool

const (
	ScanForward  ScanIndexOrderInputType = true
	ScanBackward ScanIndexOrderInputType = false
)

func Must[T any](item T, err error) T {
	if err != nil {
		panic(err)
	}
	return item
}

// // build projection expression for name list
// // NewProjectionExpressionBuilderForNames("tenant","env_id") is equal to
// // expression.NamesList(expression.Name("tenant"),expression.Name("env_id"),)
func NewProjectionExpressionBuilderForNames(names ...string) (builder expression.ProjectionBuilder, err error) {
	if len(names) == 0 {
		return builder, fmt.Errorf("cant create projection expression for empty name list")
	}
	nameBuilders := make([]expression.NameBuilder, len(names)) //[]expression.NameBuilder{}
	for i, name := range names {
		nameBuilders[i] = expression.Name(name)
	}
	return expression.NamesList(nameBuilders[0], nameBuilders[1:]...), nil
}

func Value(item any) expression.ValueBuilder {
	return expression.ValueWithOptions(item, func(vbo *expression.ValueBuilderOptions) {
		vbo.EncoderOptions = append(vbo.EncoderOptions, func(eo *attributevalue.EncoderOptions) {
			eo.EncodeTime = encodeTimeRfc339
		})
	})
}

// type UpdateItemInputForTableOpts struct {
// 	ErrorAttemptToUpdateKey      bool //if true query will fail if the item encoding contains the primary key or range key of the table if false the keys will be dropped from the update query without error
// 	EncoderOpts                  attributevalue.EncoderOptions
// 	ConditionalExpressionBuilder expression.ConditionBuilder
// }

// func NewUpdateItemInputForTable[
// 	KeyType Keyable[HashKeyType, RangeKeyType],
// 	HashKeyType AttributeValueKeyType,
// 	RangeKeyType AttributeValueKeyType,
// ](
// 	table Table[KeyType, HashKeyType, RangeKeyType],
// 	object KeyType,
// 	optsFns ...FuncOption[UpdateItemInputForTableOpts]) (queryInput dynamodb.UpdateItemInput, err error) {

// 	keyAttributes, err := table.EncodeKey(object)
// 	if err != nil {
// 		return
// 	}
// 	opts := UpdateItemInputForTableOpts{}

// 	ApplyOptions(&opts, optsFns)

// 	//marshal the data to update
// 	av, err := attributevalue.MarshalMapWithOptions(object, func(eo *attributevalue.EncoderOptions) {
// 		eo.EncodeTime = encodeTimeRfc339
// 	})
// 	if err != nil {
// 		return queryInput, err
// 	}
// 	builder := expression.NewBuilder()
// 	updateBuilder := expression.UpdateBuilder{}
// 	counter := 0
// 	//build update condition from marshaled data
// 	for encodedAttrName, encodedAttrVal := range av {
// 		if encodedAttrName == table.primaryKeyFieldName || encodedAttrName == table.rangeKeyFieldName {
// 			if opts.ErrorAttemptToUpdateKey {
// 				return queryInput, fmt.Errorf("failed to encode query it contains the table hash key %v", table.primaryKeyFieldName)
// 			}
// 			continue
// 		}
// 		if encodedAttrName == table.rangeKeyFieldName {
// 			if opts.ErrorAttemptToUpdateKey {
// 				return queryInput, fmt.Errorf("failed to encode query  it contains the table range key %v", table.rangeKeyFieldName)
// 			}
// 			continue
// 		}
// 		name := expression.Name(encodedAttrName)
// 		val := Value(encodedAttrVal)
// 		updateBuilder.Set(name, val)
// 		counter++
// 	}

// 	if counter == 0 {
// 		return queryInput, fmt.Errorf("there is nothing to update in the query")
// 	}
// 	opts.ConditionalExpressionBuilder = addAttributesExistsToQueryBuilder(opts.ConditionalExpressionBuilder, keyAttributes)

// 	builder = builder.WithUpdate(updateBuilder)
// 	if opts.ConditionalExpressionBuilder.IsSet() {
// 		builder = builder.WithCondition(opts.ConditionalExpressionBuilder)
// 	}

// 	expr, err := builder.Build()
// 	if err != nil {
// 		return queryInput, err
// 	}

// 	queryInput.Key = keyAttributes
// 	queryInput.TableName = aws.String(table.TableName)

// 	queryInput.ConditionExpression = expr.Condition()
// 	queryInput.ExpressionAttributeNames = expr.Names()
// 	queryInput.ExpressionAttributeValues = expr.Values()
// 	queryInput.UpdateExpression = expr.Update()
// 	queryInput.ReturnValues = types.ReturnValueAllNew
// 	queryInput.ReturnValuesOnConditionCheckFailure = types.ReturnValuesOnConditionCheckFailureAllOld

// 	return queryInput, nil
// }

// /////////////////////////////////////////////////////
var _ Keyable[string, uint64] = jobInternalV2{}

type JobsTableKeyProvider struct {
	Tenant string
	//range key
	JobID uint64
}

func (j JobsTableKeyProvider) HashKey() (attr string, isValid bool) {
	return j.Tenant, j.Tenant != ""
}

func (j JobsTableKeyProvider) RangeKey() (attr uint64, isValid bool) {
	return j.JobID, j.JobID != 0
}

type jobInternalV2 struct {
	Tenant string
	JobID  uint64
	State  string
	Type   string
}

func (j jobInternalV2) HashKey() (attr string, isValid bool) {
	return j.Tenant, j.Tenant != ""
}

func (j jobInternalV2) RangeKey() (attr uint64, isValid bool) {
	return j.JobID, j.JobID != 0
}

type jobStateInternal jobInternalV2

type jobStateKeyProvider struct {
	Tenant string
	State  string
}

func (j jobStateKeyProvider) HashKey() (attr string, isValid bool) {
	return j.Tenant, j.Tenant != ""
}

func (j jobStateKeyProvider) RangeKey() (attr string, isValid bool) {
	return j.State, j.State != ""
}

type JobTypeKeyProvider struct {
	Tenant string
	Type   string
	//range key
	JobID uint64
}

func (j JobTypeKeyProvider) HashKey() (attr string, isValid bool) {
	if j.Tenant == "" {
		return attr, false
	}
	if j.Type == "" {
		return attr, false
	}
	return j.Tenant + "#" + j.Type, true
}

func (j JobTypeKeyProvider) RangeKey() (attr int64, isValid bool) {
	return int64(j.JobID), j.JobID != 0
}

var JobsTableV2_EXAMPLE = struct {
	Table[JobsTableKeyProvider, string, uint64]
	JobStateIndex LocalSecondaryIndex[jobStateKeyProvider, string, string]
	JobTypeIndex  GlobalSecondaryIndex[JobTypeKeyProvider, string, int64]
}{
	Table: Must(NewTable[JobsTableKeyProvider](
		TableIndexDescriptor{
			TableName:           "jobs",
			primaryKeyFieldName: "tenant",
			rangeKeyFieldName:   "jobID",
		})),

	JobStateIndex: Must(
		NewLocalSecondaryIndex[jobStateKeyProvider](
			TableIndexDescriptor{
				TableName:           "jobs",
				primaryKeyFieldName: "tenant",
				rangeKeyFieldName:   "state",
				indexName:           "jobstateIndex",
			}),
	),
	JobTypeIndex: Must(
		NewGlobalSecondaryIndex[JobTypeKeyProvider](
			TableIndexDescriptor{
				TableName:           "jobs",
				primaryKeyFieldName: "tenant",
				rangeKeyFieldName:   "jobType",
				indexName:           "jobTypeIndex",
			}),
	),
}

func examples() {
	//TODO: otel integration https://pkg.go.dev/github.com/aws/smithy-go/metrics/smithyotelmetrics
	var client *dynamodb.Client //TODO: create real client
	//query to get single item for the tenant
	//will fail to encode the item since jobID is missing
	_, err := NewGetItemInputForTable(JobsTableV2_EXAMPLE.Table, JobsTableKeyProvider{Tenant: "my_tenant"})
	if err == nil {
		fmt.Print("expected to fail to create query - missing range key for table that have range key")
	}

	input, err := NewGetItemInputForTable(JobsTableV2_EXAMPLE.Table, JobsTableKeyProvider{Tenant: "my_tenant", JobID: 600})
	if err != nil {
		fmt.Print(err)
	}

	res, err := ExecuteGetItem[jobInternalV2](context.TODO(), client, &input)
	if err != nil {
		fmt.Print(err)
	} else {
		fmt.Println("got result", res)
	}

	//same query as before with options to get item without consistency
	NewGetItemInputForTable(JobsTableV2_EXAMPLE.Table, JobsTableKeyProvider{Tenant: "my_tenant", JobID: 600}, func(opts *GetItemQueryOpts) { opts.ConsistentRead = false })

	//put item query
	///
	putInput, err := NewPutNewItemInput(JobsTableV2_EXAMPLE.Table, jobInternalV2{Tenant: "my_tenant", JobID: 10}, func(opts *PutNewItemQueryOpts) {
		opts.ReturnValuesOnConditionCheckFailure = false //extra option that will not return the item in the error if the item already exists (ConditionalCheckError)
	})
	if err != nil {
		fmt.Print("failed to encode object....")
	}
	err = ExecutePutNewItem(context.TODO(), client, &putInput)
	if err != nil {
		fmt.Print("failed to put object....")
	}

	//same as  NewPutNewItemInput+ ExecutePutNewItem
	//this time it will not set the optionall option and return the item in the error incase there is conditionalCheck failure
	PutNewItem(context.TODO(), client, JobsTableV2_EXAMPLE.Table, JobsTableKeyProvider{Tenant: "my_tenant", JobID: 600})

	// //get all jobs for tenant with jobID between 200 to 600 (getJobsPaged)
	queryInput, _ := NewQueryItemsInputForTable(JobsTableV2_EXAMPLE.Table, JobsTableKeyProvider{Tenant: "my_tenant"}, func(jobID expression.KeyBuilder) expression.KeyConditionBuilder {
		return expression.KeyBetween(jobID, expression.Value(200), expression.Value(600))
	})
	//build query to get all running jobs for tenant
	queryInput, _ = NewQueryItemsInputForLSI(JobsTableV2_EXAMPLE.JobStateIndex, jobStateKeyProvider{Tenant: "my_tennat"}, func(jobState expression.KeyBuilder) expression.KeyConditionBuilder {
		return jobState.Equal(expression.Value(types.AttributeValueMemberS{Value: "running"}))
	})
	//get single page
	jobs, nextPageToken, err := ExecuteQuerySinglePage[jobInternalV2](context.Background(), client, &queryInput)
	if err != nil {
		print(err)
	} else {
		println("got result", jobs, "next page token", nextPageToken)
	}

	queryInput, _ = NewQueryItemsInputForGSI(JobsTableV2_EXAMPLE.JobTypeIndex, JobTypeKeyProvider{Tenant: "my_tenant", Type: "Node_Setup"}, nil)
	iter := NewQueryItemsIterator[jobInternalV2](client, &queryInput)
	for i := 0; i < 10 && iter.HasMorePages(); {
		//example to call iterator
		//with custom dynamodb api options by increasing the retry attempts for this query only ins
		var page []jobInternalV2
		page, err = iter.NextPage(context.TODO(), func(o *dynamodb.Options) { o.RetryMaxAttempts = 20 })
		if err != nil {
			fmt.Print(err)
			break
		}
		fmt.Println("got page", i, "page:", page)
	}

	//create delete query+execute it

	deleteInput, checker, err := NewDeleteItemInputForTable(JobsTableV2_EXAMPLE.Table, JobsTableKeyProvider{Tenant: "my_tenant", JobID: 10})
	if err == nil {
		err = ExecuteDeleteItem(context.Background(), client, &deleteInput, checker)
		fmt.Println("delete item result", err)
	}

	//same thing....
	err = DeleteItem(context.Background(), client, JobsTableV2_EXAMPLE.Table, JobsTableKeyProvider{Tenant: "my_tenant", JobID: 10})
	fmt.Println("delete item result=", err)

	//create other query this time ExecuteDeleteItemAndReturnOldValue will be used to get the deleted item value
	//also specify to not fail if item doesnt exists(nil item will be returned from ExecuteDeleteItemAndReturnOldValue in this case)
	deleteInput, checker, err = NewDeleteItemInputForTable(JobsTableV2_EXAMPLE.Table, JobsTableKeyProvider{Tenant: "my_tenant", JobID: 10}, func(opts *DeleteItemInputForTableOpts) {
		opts.CheckForItemDoesntExistsErr = false
	})
	if err == nil {
		oldItemPtr, err := ExecuteDeleteItemAndReturnOldValue[map[string]any](context.Background(), client, &deleteInput, checker)
		fmt.Println("delete item result", err, oldItemPtr)
	}

}

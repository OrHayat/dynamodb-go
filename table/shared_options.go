package table

import (
	"reflect"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/orhayat/dynamodb-go/serializer"
)

// decoderOption....

func WithDecoder(decoder *serializer.Decoder) DecoderOption {
	return DecoderOption{
		Decoder: decoder,
	}
}

var _ GetItemOptions = DecoderOption{}
var _ BatchGetItemOptions = DecoderOption{}
var _ ScanOptions = DecoderOption{}

type DecoderOption struct {
	Decoder *serializer.Decoder
}

// applyScanOption implements ScanOptions.
func (o DecoderOption) applyScanOption(cfg *ScanConfig) {
	cfg.Decoder = o.Decoder
}

// applyBatchGetItemOption implements BatchGetItemOptions.
func (o DecoderOption) applyBatchGetItemOption(cfg *BatchGetItemConfig) {
	cfg.Decoder = o.Decoder
}

func (o DecoderOption) applyGetItemOption(cfg *GetItemConfig) {
	cfg.Decoder = o.Decoder
}

// encoderOption....

func WithEncoder(encoder *serializer.Encoder) EncoderOption {
	return EncoderOption{
		Encoder: encoder,
	}
}

var _ PutItemOptions = EncoderOption{}
var _ BatchWriteItemOptions = EncoderOption{}

type EncoderOption struct {
	Encoder *serializer.Encoder
}

// applyBatchWriteItems implements BatchWriteItemOptions.
func (e EncoderOption) applyBatchWriteItems(cfg *BatchWriteItemConfig) {
	cfg.Encoder = e.Encoder
}

// applyPutItemOption implements PutItemOptions.
func (e EncoderOption) applyPutItemOption(cfg *PutItemConfig) {
	cfg.Encoder = e.Encoder
}

// upsertOption....

type UpsertOption bool

func WithUpsert(allowUpsert bool) UpsertOption {
	return UpsertOption(allowUpsert)
}

var _ PutItemOptions = UpsertOption(false)

func (o UpsertOption) applyPutItemOption(cfg *PutItemConfig) {
	cfg.AllowUpsert = bool(o)
}

// consistencyOption....

var _ GetItemOptions = ConsistencyOption(false)

type ConsistencyOption bool

func (o ConsistencyOption) applyGetItemOption(cfg *GetItemConfig) {
	cfg.Consistency = bool(o)
}

func WithConsistency(consistency bool) ConsistencyOption {
	return ConsistencyOption(consistency)
}

func WithLimit(limit int) LimitOption {
	return LimitOption{
		Limit: limit,
	}
}

var _ ScanOptions = LimitOption{}

type LimitOption struct {
	Limit int
}

// applyScanOption implements ScanOptions.
func (o LimitOption) applyScanOption(cfg *ScanConfig) {
	if o.Limit > 0 {
		cfg.Limit = aws.Int32(int32(o.Limit))
	}
}

type ScanKeysOrder int

const (
	ScanKeysOrderUndefined ScanKeysOrder = iota
	ScanKeysOrderAscending
	ScanKeysOrderDescending
)

var _ QueryOptions = ScanKeysOrder(0)

func WithScanKeysOrder(order ScanKeysOrder) ScanKeysOrder {
	return order
}

func (o ScanKeysOrder) applyQueryOption(cfg *QueryConfig) {
	cfg.ScanKeysOrder = o
}

// ------------ FilterOption ----------------
func WithFilter(filterExpression expression.ConditionBuilder) FilterOption {
	return FilterOption{
		FilterExpression: filterExpression,
	}
}

var _ QueryOptions = FilterOption{}
var _ ScanOptions = FilterOption{}

type FilterOption struct {
	FilterExpression expression.ConditionBuilder
}

// applyScanOption implements ScanOptions.
func (o FilterOption) applyScanOption(cfg *ScanConfig) {
	cfg.FilterExpression = o.FilterExpression
}

// applyQueryOption implements QueryOptions.
func (o FilterOption) applyQueryOption(cfg *QueryConfig) {
	cfg.FilterExpression = o.FilterExpression
}

// ------------- ProjectionOption ----------------

// WithProjection returns a ProjectionOption that sets the Projection Expression
// to the specified ProjectionBuilder. The ProjectionBuilder can be constructed
// using functions from the expression package, such as NamesList() and AddNames().
//
// Example:
//
//	// projection represents the list of names {"foo", "bar"}
//	projection := expression.NamesList(expression.Name("foo"), expression.Name("bar"))
//
//	// Used to make an Builder
//	builder := expression.NewBuilder().WithProjection(projection)
//
// Expression Equivalent:
//
//	expression.NamesList(expression.Name("foo"), expression.Name("bar"))
//	"foo, bar"
func WithProjection(projection expression.ProjectionBuilder) ProjectionOption {
	return ProjectionOption{
		ProjectionExpression: projection,
		projectionEnabled:    true,
	}
}

// WithProjectionForNameList returns a ProjectionOption that includes the specified
// attribute names. The first argument is required, and any additional names can
// be provided as variadic arguments.
//
// Example:
//
//	// projection includes the attributes "foo", "bar", and "baz"
//	projection := table.WithProjectionForNameList("foo", "bar", "baz")
//
// Note: If no names are provided, the resulting ProjectionOption will not enable
// projection.
//
// Expression Equivalent:
//
//	expression.NamesList(expression.Name("foo"), expression.Name("bar"), expression.Name("baz"))
//	"foo, bar, baz"
func WithProjectionForNameList(name string, names ...string) ProjectionOption {
	res := expression.ProjectionBuilder{}
	for _, n := range names {
		res = expression.AddNames(res, expression.Name(n))
	}
	return ProjectionOption{
		ProjectionExpression: res,
		projectionEnabled:    len(names) > 0,
	}
}

var structProjectionCache = sync.Map{}

type structProjectionCacheKey struct {
	tagName string
	t       reflect.Type
}

func getStructProjection[T any](tagName string) ProjectionOption {
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		panic("getStructProjection only supports struct types")
	}
	key := structProjectionCacheKey{
		tagName: tagName,
		t:       t,
	}
	if v, ok := structProjectionCache.Load(key); ok {
		return v.(ProjectionOption)
	}
	// not in cache, compute it
	nameList := []string{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tagName := field.Tag.Get(tagName)
		if tagName == "" {
			nameList = append(nameList, field.Name)
			continue
		}
		if tagName == "-" {
			continue
		}
		name, _ := strings.CutSuffix(tagName, ",")
		nameList = append(nameList, name)
	}
	var opt ProjectionOption
	if len(nameList) == 0 {
		opt = ProjectionOption{}
	} else {
		opt = WithProjectionForNameList(nameList[0], nameList[1:]...)
	}
	res, _ := structProjectionCache.LoadOrStore(key, opt)
	return res.(ProjectionOption)

}

// WithProjectionForType returns a ProjectionOption that includes all struct fields
// of the specified struct type T. The struct tags with the specified tagName are
// used to determine the attribute names. If a field does not have the specified
// tag, the field name is used as the attribute name. Fields with a tag value of
// "-" are ignored.
//
// Example:
//
//	type Item struct {
//	    ID   string `dynamodb:"id"`
//	    Name string `dynamodb:"name"`
//		IgnoredField string `dynamodb:"-"`
//	    Age  int
//	}
//
//	// projection includes "id", "name", and "Age"
//	projection := table.WithProjectionForType[Item]("dynamodb")
//
// Note: The result is cached for each unique combination of struct type T and
// tagName, so subsequent calls with the same type and tag will be efficient.
//
// Panics if T is not a struct type.
func WithProjectionForType[T any](tagName string) ProjectionOption {
	return getStructProjection[T](tagName)
}

var _ GetItemOptions = ProjectionOption{}
var _ BatchGetItemOptions = ProjectionOption{}

type ProjectionOption struct {
	ProjectionExpression expression.ProjectionBuilder
	projectionEnabled    bool
}

// applyBatchGetItemOption implements BatchGetItemOptions.
func (o ProjectionOption) applyBatchGetItemOption(cfg *BatchGetItemConfig) {
	cfg.projectionBuilder = o.ProjectionExpression
	cfg.projectionEnabled = o.projectionEnabled
}

func (o ProjectionOption) applyGetItemOption(cfg *GetItemConfig) {
	cfg.projectionBuilder = o.ProjectionExpression
	cfg.projectionEnabled = o.projectionEnabled
}

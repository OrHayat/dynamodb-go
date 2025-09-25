package table

import (
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

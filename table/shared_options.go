package table

import "github.com/orhayat/dynamodb-go/serializer"

// decoderOption....

func WithDecoder(decoder *serializer.Decoder) DecoderOption {
	return DecoderOption{
		Decoder: decoder,
	}
}

var _ GetItemOptions = DecoderOption{}

type DecoderOption struct {
	Decoder *serializer.Decoder
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

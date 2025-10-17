package table

import (
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
var _ TransactionGetItemOptions = DecoderOption{}

type DecoderOption struct {
	Decoder *serializer.Decoder
}

// applyTransactionGetItemOption implements TransactionGetItemOptions.
func (o DecoderOption) applyTransactionGetItemOption(cfg *TransactionGetItemConfig) {
	cfg.Decoder = o.Decoder
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
var _ TransactionWriteItemOptions = EncoderOption{}

type EncoderOption struct {
	Encoder *serializer.Encoder
}

// applyTransactionWriteItemOption implements TransactionWriteItemOptions.
func (e EncoderOption) applyTransactionWriteItemOption(cfg *TransactionWriteItemConfig) {
	cfg.Encoder = e.Encoder
}

// applyPutItemOption implements PutItemOptions.
func (e EncoderOption) applyPutItemOption(cfg *PutItemConfig) {
	cfg.Encoder = e.Encoder
}

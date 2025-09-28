package table

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/orhayat/dynamodb-go/serializer"
)

// Default encoder and decoder that this package use internally if client does not set one
// can be override by setting client encoder/decoder or by passing encoder/decoder in the options of the requests
var s_encoder = serializer.NewEncoder()
var s_decoder = serializer.NewDecoder()

func NewClient(client *dynamodb.Client, encoder *serializer.Encoder, decoder *serializer.Decoder) (*Client, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}
	return &Client{
		encoder: encoder,
		decoder: decoder,
		Client:  client,
	}, nil
}

type Client struct {
	decoder *serializer.Decoder
	encoder *serializer.Encoder
	*dynamodb.Client
}

func (c *Client) GetDecoder() *serializer.Decoder {
	return c.decoder
}

func (c *Client) GetEncoder() *serializer.Encoder {
	return c.encoder
}

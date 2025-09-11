package table

// import "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/serializer"
)

type TableDefinition struct {
	Name       string
	PrimaryKey AttributeDefinition
	RangeKey   AttributeDefinition
}

func (d *TableDefinition) ExtractKeys(encodedObject map[string]types.AttributeValue) (pk types.AttributeValue, sk types.AttributeValue, err error) {
	pkVal := encodedObject[d.PrimaryKey.Name]
	if pkVal == nil {
		err = errors.Join(err, fmt.Errorf("encoded object:missing primary key %q", d.PrimaryKey.Name))
	}
	var skVal types.AttributeValue
	if d.RangeKey.Name != "" {
		skVal = encodedObject[d.RangeKey.Name]
		if skVal == nil {
			err = errors.Join(err, fmt.Errorf("encoded object:missing range key %q", d.RangeKey.Name))
		}
	}
	return pkVal, skVal, err
}
func (d *TableDefinition) encodedKeyToVal(k types.AttributeValue) any {
	if k == nil {
		return nil
	}
	switch v := k.(type) {
	case *types.AttributeValueMemberS:
		return v.Value
	case *types.AttributeValueMemberN:
		return v.Value
	case *types.AttributeValueMemberB:
		return v.Value
	default:
		//unreachable
		panic(fmt.Sprintf("unreachable:nsupported key type %T", k))
	}
}
func (d *TableDefinition) getKey(primaryKey any, sortkey any) (res map[string]types.AttributeValue, err error) {

	var count = 2
	if d.RangeKey.Name == "" {
		count = 1
	}
	res = make(map[string]types.AttributeValue, count)
	av, err := d.PrimaryKey.encodeToAv(primaryKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode table primary key")
	}
	res[d.PrimaryKey.Name] = av
	if d.RangeKey.Name != "" {
		av, err = d.RangeKey.encodeToAv(sortkey)
		if err != nil {
			return nil, fmt.Errorf("failed to encode table range key")
		}
		res[d.RangeKey.Name] = av

	}
	return res, nil
}

type AttributeDefinition struct {
	Name string
	Type types.ScalarAttributeType
}

func (ad AttributeDefinition) encodeToAv(item any) (types.AttributeValue, error) {
	if ad.Name == "" {
		return nil, nil
	}
	encoded, err := s_encoder.Marshal(item)
	if err != nil {
		return nil, err
	}
	switch ad.Type {
	case types.ScalarAttributeTypeS:
		_, ok := encoded.(*types.AttributeValueMemberS)
		if !ok {
			return nil, fmt.Errorf("encoded attribute to %T and not to string", item)
		}
	case types.ScalarAttributeTypeN:
		_, ok := encoded.(*types.AttributeValueMemberN)
		if !ok {
			return nil, fmt.Errorf("encoded attribute to %T and not to number", item)

		}
	case types.ScalarAttributeTypeB:
		_, ok := encoded.(*types.AttributeValueMemberB)
		if !ok {
			return nil, fmt.Errorf("encoded attribute to %T and not to bool", item)
		}
	default:
		return nil, fmt.Errorf("unsopported item type %T", item)
	}
	return encoded, nil
}

// Default encoder and decoder if client not set
// can be override by setting client encoder/decoder or by passing encoder/decoder in the options of the requests
var s_encoder = serializer.NewEncoder()
var s_decoder = serializer.NewDecoder()

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

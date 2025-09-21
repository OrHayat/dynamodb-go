package table

// import "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/serializer"
)

type Key struct {
	PK any
	SK any
}

type Billing struct {
	BillingMode           types.BillingMode
	ProvisionedThroughput *types.ProvisionedThroughput //only for PROVISIONED mode
	OnDemandThroughput    *types.OnDemandThroughput    //only for PAY_PER_REQUEST mode
}

/*
	type LocalSecondaryIndex struct {
		IndexName string
		RangeKey  AttributeDefinition
	}

	type GlobalSecondaryIndex struct {
		IndexName  string
		PrimaryKey AttributeDefinition
		RangeKey   AttributeDefinition
	}

	type TableDefinition struct {
		Name       string
		PrimaryKey AttributeDefinition
		RangeKey   AttributeDefinition
		Billing
	}
*/

type LocalSecondaryIndex struct {
	IndexName string
	RangeKey  AttributeDefinition
}

func (lsi *LocalSecondaryIndex) getKey(table *TableDefinition, key Key) (res map[string]types.AttributeValue, err error) {

	res = make(map[string]types.AttributeValue, 2)
	res[table.PrimaryKey.Name], err = table.PrimaryKey.encodeToAv(key.PK)
	if err != nil {
		return nil, fmt.Errorf("failed to encode table primary key")
	}
	if lsi.RangeKey.Name == "" {
		return nil, fmt.Errorf("LSI must have range key")
	}
	res[lsi.RangeKey.Name], err = lsi.RangeKey.encodeToAv(key.SK)
	if err != nil {
		return nil, fmt.Errorf("failed to encode index range key")
	}
	return res, nil
}

type GlobalSecondaryIndex struct {
	IndexName  string
	PrimaryKey AttributeDefinition //required
	RangeKey   AttributeDefinition //optional
}

func (gsi *GlobalSecondaryIndex) getKey(key Key) (res map[string]types.AttributeValue, err error) {

	var count = 2
	if gsi.RangeKey.Name == "" {
		count = 1
	}
	res = make(map[string]types.AttributeValue, count)
	av, err := gsi.PrimaryKey.encodeToAv(key.PK)
	if err != nil {
		return nil, fmt.Errorf("failed to encode table primary key")
	}
	res[gsi.PrimaryKey.Name] = av
	if gsi.RangeKey.Name != "" {
		av, err = gsi.RangeKey.encodeToAv(key.SK)
		if err != nil {
			return nil, fmt.Errorf("failed to encode table range key")
		}
		res[gsi.RangeKey.Name] = av
	}
	return res, nil
}

type TableDefinition struct {
	Name       string
	PrimaryKey AttributeDefinition
	RangeKey   AttributeDefinition
	GSI        []GlobalSecondaryIndex
	LSI        []LocalSecondaryIndex
	Billing
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

// getKeyForIndex returns the encoded key for the index if its provided and the table key if indexName is empty
func (d *TableDefinition) getKeyForIndex(
	indexName string, key Key,
) (res map[string]types.AttributeValue, err error) {
	if indexName == "" {
		return d.getKey(key)
	}
	for _, gsi := range d.GSI {
		if gsi.IndexName == indexName {
			return gsi.getKey(key)
		}
	}
	for _, lsi := range d.LSI {
		if lsi.IndexName == indexName {
			return lsi.getKey(d, key)
		}
	}

	return nil, fmt.Errorf("index %q not found", indexName)
}

func (d *TableDefinition) getKey(key Key) (res map[string]types.AttributeValue, err error) {

	var count = 2
	if d.RangeKey.Name == "" {
		count = 1
	}
	res = make(map[string]types.AttributeValue, count)
	av, err := d.PrimaryKey.encodeToAv(key.PK)
	if err != nil {
		return nil, fmt.Errorf("failed to encode table primary key")
	}
	res[d.PrimaryKey.Name] = av
	if d.RangeKey.Name != "" {
		av, err = d.RangeKey.encodeToAv(key.SK)
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

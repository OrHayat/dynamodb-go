package table

// import "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/serializer"
)

type TableDefinition struct {
	Name       string
	PrimaryKey AttributeDefinition
	RangeKey   AttributeDefinition
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
		res[d.PrimaryKey.Name] = av

	}
	return res, nil
}

type AttributeDefinition struct {
	Name string
	Type types.ScalarAttributeType
}

var s_encoder = serializer.NewEncoder()

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

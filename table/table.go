package table

// import "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Key struct {
	PK any
	SK any
}

// exactly one of the fields should be set empty struct will be used to indicate no pagination key and start from beginning of the table/index
type PaginationKey struct {
	encodedKey map[string]types.AttributeValue
	useUserKey bool
	userKey    Key
}

func (p PaginationKey) resolveExclusiveStartKey(table *TableDefinition, indexName string) (map[string]types.AttributeValue, error) {
	if p.useUserKey {
		return table.getKeyForIndex(indexName, p.userKey)
	}
	return p.encodedKey, nil
}

func NewPaginationKey(key Key) PaginationKey {
	return PaginationKey{
		useUserKey: true,
		userKey:    key,
	}
}

// HasMore returns true if there are more pages to fetch
func (p PaginationKey) HasMore() bool {
	return len(p.encodedKey) > 0 || p.useUserKey
}

type Billing struct {
	BillingMode           types.BillingMode
	ProvisionedThroughput *types.ProvisionedThroughput //only for PROVISIONED mode
	OnDemandThroughput    *types.OnDemandThroughput    //only for PAY_PER_REQUEST mode
}

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
	Billing    Billing
}

func (d *TableDefinition) getPkName(index string) (string, error) {
	if index == "" {
		return d.PrimaryKey.Name, nil
	}
	if gsi := d.getGSI(index); gsi != nil {
		return gsi.PrimaryKey.Name, nil
	}
	if lsi := d.getLSI(index); lsi != nil {
		return d.PrimaryKey.Name, nil
	}
	return "", fmt.Errorf("index %q not found", index)
}

func (d *TableDefinition) getSkName(index string) (string, error) {
	if index == "" {
		return d.RangeKey.Name, nil
	}
	if gsi := d.getGSI(index); gsi != nil {
		return gsi.RangeKey.Name, nil
	}
	if lsi := d.getLSI(index); lsi != nil {
		return lsi.RangeKey.Name, nil
	}
	return "", fmt.Errorf("index %q not found", index)
}

func (d *TableDefinition) getSKName(index string) (string, error) {
	if index == "" {
		return d.RangeKey.Name, nil
	}
	if gsi := d.getGSI(index); gsi != nil {
		return gsi.RangeKey.Name, nil
	}
	if lsi := d.getLSI(index); lsi != nil {
		return lsi.RangeKey.Name, nil
	}
	return "", fmt.Errorf("index %q not found", index)
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
		panic(fmt.Sprintf("unreachable:unsupported key type %T", k))
	}
}

// func (d *TableDefinition) encodeForQuery(){}

// getKeyForIndex returns the encoded key for the index if its provided and the table key if indexName is empty
func (d *TableDefinition) getKeyForIndex(
	indexName string, key Key,
) (res map[string]types.AttributeValue, err error) {
	if indexName == "" {
		return d.getKey(key)
	}
	if gsi := d.getGSI(indexName); gsi != nil {
		return gsi.getKey(key)
	}

	if lsi := d.getLSI(indexName); lsi != nil {
		return lsi.getKey(d, key)
	}

	return nil, fmt.Errorf("index %q not found", indexName)
}

func (d *TableDefinition) getPkForIndex(
	indexName string, pk any,
) (res types.AttributeValue, err error) {
	if indexName == "" {
		return d.PrimaryKey.encodeToAv(pk)
	}
	if gsi := d.getGSI(indexName); gsi != nil {
		return gsi.PrimaryKey.encodeToAv(pk)
	}
	if lsi := d.getLSI(indexName); lsi != nil {
		return d.PrimaryKey.encodeToAv(pk)
	}
	return nil, fmt.Errorf("index %q not found", indexName)
}

func (d *TableDefinition) getSkForIndex(
	indexName string, sk any,
) (res types.AttributeValue, err error) {
	if indexName == "" {
		return d.RangeKey.encodeToAv(sk)
	}
	if gsi := d.getGSI(indexName); gsi != nil {
		return gsi.RangeKey.encodeToAv(sk)
	}
	if lsi := d.getLSI(indexName); lsi != nil {
		return lsi.RangeKey.encodeToAv(sk)
	}
	return nil, fmt.Errorf("index %q not found", indexName)
}

func (d *TableDefinition) getGSI(indexName string) *GlobalSecondaryIndex {
	for i, gsi := range d.GSI {
		if gsi.IndexName == indexName {
			return &d.GSI[i]
		}
	}
	return nil
}

func (d *TableDefinition) getLSI(indexName string) *LocalSecondaryIndex {
	for i, lsi := range d.LSI {
		if lsi.IndexName == indexName {
			return &d.LSI[i]
		}
	}
	return nil
}

// getKey returns the encoded key for the table
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
	switch ad.Type {
	case types.ScalarAttributeTypeS:
		return encodeString(ad.Name, item)
	case types.ScalarAttributeTypeB:
		return encodeBytes(ad.Name, item)
	case types.ScalarAttributeTypeN:
		return encodeNumber(ad.Name, item)
	default:
		return nil, fmt.Errorf("unsupported attribute type %s for attribute %s", ad.Type, ad.Name)
	}
}

func encodeString(attributeName string, item any) (types.AttributeValue, error) {
	val := reflect.ValueOf(item)
	if val.Kind() == reflect.String {
		return &types.AttributeValueMemberS{Value: val.String()}, nil
	}
	return nil, fmt.Errorf("cannot convert item of type %T to dynamoDB string for attribute %s", item, attributeName)
}

func encodeBytes(attributeName string, item any) (types.AttributeValue, error) {
	casted, ok := item.([]byte)
	if ok {
		return &types.AttributeValueMemberB{Value: casted}, nil
	}
	return nil, fmt.Errorf("cannot convert item of type %T to dynamoDB byte blob for attribute %s", item, attributeName)
}

func encodeNumber(attributeName string, item any) (types.AttributeValue, error) {
	val := reflect.ValueOf(item)
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &types.AttributeValueMemberN{Value: strconv.FormatInt(val.Int(), 10)}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &types.AttributeValueMemberN{Value: strconv.FormatUint(val.Uint(), 10)}, nil
	case reflect.Float32, reflect.Float64:
		return &types.AttributeValueMemberN{Value: strconv.FormatFloat(val.Float(), 'f', -1, val.Type().Bits())}, nil
	}
	return nil, fmt.Errorf("cannot convert item of type %T to dynamoDB number for attribute %s", item, attributeName)
}

func ensureKeyNotExists(table *TableDefinition) expression.ConditionBuilder {
	condition := expression.AttributeNotExists(expression.Name(table.PrimaryKey.Name))
	if table.RangeKey.Name != "" {
		condition = condition.And(expression.AttributeNotExists(expression.Name(table.RangeKey.Name)))
	}
	return condition
}

func ensureKeyExists(table *TableDefinition) expression.ConditionBuilder {
	condition := expression.AttributeExists(expression.Name(table.PrimaryKey.Name))
	if table.RangeKey.Name != "" {
		condition = condition.And(expression.AttributeExists(expression.Name(table.RangeKey.Name)))
	}
	return condition
}

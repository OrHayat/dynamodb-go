package table

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type undefinedKeyType = struct{} //use this type to mark GSI/Table RangeKeyType if the index don't need range key

// constraint on hash key and sort key types  <types.ScalarAttributeType>(number,int,binary)
type AttributeValueKeyType = interface {
	~string | ~uint64 | ~int64 | ~int | ~uint | ~[]byte | undefinedKeyType
}

type HashKeyProvider[KeyType any, HashKeyType AttributeValueKeyType] interface {
	HashKey() (attr HashKeyType, isValid bool)
}

type Keyable[HashKeyType AttributeValueKeyType, RangeKeyType AttributeValueKeyType] interface {
	HashKey() (attr HashKeyType, isValid bool)
	RangeKey() (attr RangeKeyType, isValid bool)
}

type TableIndexDescriptor struct {
	TableName           string
	primaryKeyFieldName string
	rangeKeyFieldName   string //can be empty on  table or GSI if the table/index dont need a range key , required in LSI
	indexName           string //keep empty for main table index fill for LSI and GSI
}

type Table[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
] struct {
	KeyEncoder[KeyType, HashKeyType, RangeKeyType]
}

func NewTable[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](tableMainIndexInfo TableIndexDescriptor) (table Table[KeyType, HashKeyType, RangeKeyType], err error) {
	if tableMainIndexInfo.indexName != "" {
		return table, fmt.Errorf("table main index index name is required to be empty")
	}
	table.KeyEncoder, err = NewKeyEncoder[KeyType](tableMainIndexInfo)
	return table, err
}

type GlobalSecondaryIndex[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
] struct {
	KeyEncoder[KeyType, HashKeyType, RangeKeyType]
}

func NewGlobalSecondaryIndex[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](indexInfo TableIndexDescriptor) (index GlobalSecondaryIndex[KeyType, HashKeyType, RangeKeyType], err error) {
	if index.indexName == "" {
		return index, fmt.Errorf("table global secondary index name is required to be not empty")
	}
	//ensure GSI has Index as suffix
	if !strings.HasSuffix("Index", indexInfo.indexName) {
		indexInfo.indexName += "Index"
	}
	index.KeyEncoder, err = NewKeyEncoder[KeyType](indexInfo)
	return index, err
}

type LocalSecondaryIndex[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
] struct {
	KeyEncoder[KeyType, HashKeyType, RangeKeyType]
}

func NewLocalSecondaryIndex[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](indexInfo TableIndexDescriptor) (index LocalSecondaryIndex[KeyType, HashKeyType, RangeKeyType], err error) {
	if indexInfo.indexName == "" {
		return index, fmt.Errorf("table %s local secondary index name is required to be not empty", indexInfo.TableName)
	}
	//ensure LSI has Index as suffix(convention)
	if !strings.HasSuffix("Index", indexInfo.indexName) {
		indexInfo.indexName += "Index"
	}
	index.KeyEncoder, err = NewKeyEncoder[KeyType](indexInfo)
	return index, err
}

// allow encoding key for given table descriptor
type KeyEncoder[
	ItemType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
] struct {
	TableIndexDescriptor
}

func EncodeValue[T AttributeValueKeyType](value T) (types.AttributeValue, bool) {
	switch v := any(value).(type) {
	case int:
		return &types.AttributeValueMemberN{Value: strconv.FormatInt(int64(v), 10)}, true
	case int64:
		return &types.AttributeValueMemberN{Value: strconv.FormatInt(int64(v), 10)}, true
	case uint:
		return &types.AttributeValueMemberN{Value: strconv.FormatUint(uint64(v), 10)}, true
	case uint64:
		return &types.AttributeValueMemberN{Value: strconv.FormatUint(uint64(v), 10)}, true
	case string:
		return &types.AttributeValueMemberS{Value: v}, true
	case []byte:
		return &types.AttributeValueMemberB{Value: v}, true
	default:
		return nil, false
	}
}

func NewKeyEncoder[
	KeyType Keyable[HashKeyType, RangeKeyType],
	HashKeyType AttributeValueKeyType,
	RangeKeyType AttributeValueKeyType,
](indexDescriptor TableIndexDescriptor) (encoder KeyEncoder[KeyType, HashKeyType, RangeKeyType], err error) {

	if indexDescriptor.TableName == "" {
		return encoder, fmt.Errorf("table name is not allowed to be empty")
	}

	if indexDescriptor.primaryKeyFieldName == "" {
		return encoder, fmt.Errorf("primary key of encoder is not allowed to be empty")
	}

	encoder = KeyEncoder[KeyType, HashKeyType, RangeKeyType]{
		TableIndexDescriptor: indexDescriptor,
	}

	return encoder, nil
}

func (e KeyEncoder[KeyType, HashKeyType, RangeKeyType]) ExtractKey(encodedObject map[string]types.AttributeValue) (key map[string]types.AttributeValue, err error) {

	attr, ok := encodedObject[e.primaryKeyFieldName]
	if !ok {
		return nil, fmt.Errorf("failed to extract primary key with name %s  from encoded object  %#v", e.primaryKeyFieldName, encodedObject)
	}

	key = map[string]types.AttributeValue{
		e.primaryKeyFieldName: attr,
	}

	//if range key is'nt needed for table/index then return hash key encoding
	if e.rangeKeyFieldName == "" {
		return key, nil
	}

	attr, ok = encodedObject[e.rangeKeyFieldName]
	if !ok {
		return nil, fmt.Errorf("failed to extract range key with name %s from encoded object  %#v", e.rangeKeyFieldName, encodedObject)
	}

	key[e.rangeKeyFieldName] = attr

	return key, nil
}
func (e KeyEncoder[KeyType, HashKeyType, RangeKeyType]) EncodeKey(item KeyType) (key map[string]types.AttributeValue, err error) {

	hashKey, ok := item.HashKey()
	if !ok {
		return nil, fmt.Errorf("invalid key of %T, hashkey cannot be empty", item)
	}
	attr, ok := EncodeValue(hashKey)
	if !ok {
		return nil, fmt.Errorf("invalid type for hashKey %T", hashKey)
	}

	key = map[string]types.AttributeValue{
		e.primaryKeyFieldName: attr,
	}

	//if range key is'nt needed for table/index then return hash key encoding
	if e.rangeKeyFieldName == "" {
		return key, nil
	}

	rangeKey, ok := item.RangeKey()
	if !ok {
		return nil, fmt.Errorf("invalid key of %T rangekey cannot be empty", item)
	}

	attr, ok = EncodeValue(rangeKey)
	if !ok {
		return nil, fmt.Errorf("invalid type for rangeKey %T", rangeKey)
	}

	//range key is valid and field name is not empty
	key[e.rangeKeyFieldName] = attr

	return key, nil
}

package table

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func newErrorNotExists(tableName string, key map[string]types.AttributeValue, err error) error {
	baseErr := fmt.Errorf("%w:item %#v was not found in table %s ,", ErrItemNotExists, key, tableName)
	if err != nil {
		return fmt.Errorf("%w:%w", baseErr, err)
	}
	return baseErr
}

func newErrorAlreadyExists(tableName string, key map[string]types.AttributeValue) error {
	return fmt.Errorf("%w:item %#v was found in table %s", ErrAlreadyExists, key, tableName)
}

var ErrAlreadyExists = errors.New("item already exists")
var ErrItemNotExists = errors.New("item doesn't exists")

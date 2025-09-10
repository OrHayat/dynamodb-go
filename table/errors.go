package table

import "errors"

var ErrItemNotFound = errors.New("item doesnt exists")
var ErrAlreadyExists = errors.New("item already exists")

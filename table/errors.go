package table

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrItemNotFound = errors.New("item doesnt exists")
var ErrAlreadyExists = errors.New("item already exists")

type operationError struct {
	operation   string
	internalErr error
	table       *TableDefinition
	pk          any
	sk          any
}

func (e *operationError) JSON() string {
	return e.ErrorJSON()
}
func (e *operationError) Text() string {
	return e.ErrorText()
}

func (e operationError) Unwrap() error {
	return e.internalErr
}

func (e *operationError) Error() string {
	sb := strings.Builder{}
	if e.operation == "" {
		//TODDO: write some generic message
	} else {
		sb.WriteString(e.operation + " failed")
	}
	if e.table != nil {
		//TODO format error
		sb.WriteString(" table" + e.table.Name)
	}

	if e.internalErr != nil {
		sb.WriteString(e.internalErr.Error())
	}
	return sb.String()
}

func (e *operationError) ErrorText() string {
	var sb strings.Builder

	// Operation context
	if e.operation == "" {
		sb.WriteString("operation failed")
	} else {
		sb.WriteString(e.operation + " failed")
	}

	// Table + key schema
	if e.table != nil {
		sb.WriteString(" on table " + e.table.Name)

		keyParts := []string{}

		if e.table.PrimaryKey.Name != "" {
			if e.pk != nil {
				keyParts = append(keyParts,
					fmt.Sprintf("pk=%s=%v", e.table.PrimaryKey.Name, e.pk))
			} else {
				keyParts = append(keyParts,
					fmt.Sprintf("pk=%s", e.table.PrimaryKey.Name))
			}
		}

		if e.table.RangeKey.Name != "" {
			if e.sk != nil {
				keyParts = append(keyParts,
					fmt.Sprintf("sk=%s=%v", e.table.RangeKey.Name, e.sk))
			} else {
				keyParts = append(keyParts,
					fmt.Sprintf("sk=%s", e.table.RangeKey.Name))
			}
		}

		if len(keyParts) > 0 {
			sb.WriteString(" {" + strings.Join(keyParts, ", ") + "}")
		}
	}

	if e.internalErr != nil {
		sb.WriteString(":" + e.internalErr.Error())
	}

	return sb.String()
}

func (e *operationError) ErrorJSON() string {
	payload := map[string]any{}
	if e.operation != "" {
		payload["operation"] = e.operation
	}
	var pkPayload map[string]any
	var skPayload map[string]any
	if e.table != nil {
		payload["table"] = e.table.Name
		if e.table.PrimaryKey.Name != "" {
			pkPayload = map[string]any{"name": e.table.PrimaryKey.Name}
		}
		if e.table.RangeKey.Name != "" {
			skPayload = map[string]any{"name": e.table.RangeKey.Name}
		}
	}
	if e.pk != nil {
		if pkPayload == nil {
			pkPayload = map[string]any{
				"value": e.pk,
			}
		} else {
			pkPayload["value"] = e.pk
		}
	}
	if e.sk != nil {
		if skPayload == nil {
			skPayload = map[string]any{
				"value": e.sk,
			}
		} else {
			skPayload["value"] = e.sk
		}
	}
	if pkPayload != nil {
		payload["pk"] = pkPayload
	}
	if skPayload != nil {
		payload["sk"] = skPayload
	}

	if e.internalErr != nil {
		payload["error"] = e.internalErr.Error()
	}

	payload = map[string]any{
		"error": payload,
	}

	b, _ := json.Marshal(payload) // ignore marshal error since map is safe
	return string(b)
}

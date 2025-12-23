package dynamo

import (
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/table"
)

// TablesListMsg is sent when ListTables completes
type TablesListMsg struct {
	Tables []string
}

// TableSchemaMsg is sent when DescribeTable completes
type TableSchemaMsg struct {
	TableName   string
	Schema      table.TableDefinition
	Description *types.TableDescription
}

// ItemsLoadedMsg is sent when Scan or Query completes
type ItemsLoadedMsg struct {
	Items       []map[string]any
	NextPage    table.PaginationKey
	HasNextPage bool
	IsQuery     bool
}

// ItemDetailMsg is sent when GetItem completes
type ItemDetailMsg struct {
	Item map[string]any
}

// ErrorMsg is sent when any operation fails
type ErrorMsg struct {
	Err       error
	Operation string
	Table     string
}

func (e ErrorMsg) Error() string {
	if e.Table != "" {
		return e.Operation + " on " + e.Table + ": " + e.Err.Error()
	}
	return e.Operation + ": " + e.Err.Error()
}

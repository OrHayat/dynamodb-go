package app

import "github.com/orhayat/dynamodb-go/table"

// View represents the current active view
type View int

const (
	ViewTablesList View = iota
	ViewTableBrowser
	ViewItemDetail
)

// Context holds shared state across views
type Context struct {
	Profile string
	Region  string

	// Current table context (set when entering browser)
	TableName   string
	TableSchema *table.TableDefinition

	// Current item (set when entering detail view)
	CurrentItem map[string]any
	ItemKey     table.Key
}

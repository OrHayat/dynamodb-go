package messages

import "github.com/orhayat/dynamodb-go/table"

// TableMode specifies the initial mode when opening a table
type TableMode int

const (
	TableModeScan TableMode = iota
	TableModeQuery
	TableModeDescribe
)

// NavigateToTableMsg tells the app to navigate to the table browser
type NavigateToTableMsg struct {
	TableName   string
	InitialMode TableMode
}

// NavigateToItemMsg tells the app to navigate to item detail
type NavigateToItemMsg struct {
	Item map[string]any
	Key  table.Key
}

// NavigateBackMsg tells the app to go back to previous view
type NavigateBackMsg struct{}

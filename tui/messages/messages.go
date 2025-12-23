package messages

import "github.com/orhayat/dynamodb-go/table"

// NavigateToTableMsg tells the app to navigate to the table browser
type NavigateToTableMsg struct {
	TableName string
}

// NavigateToItemMsg tells the app to navigate to item detail
type NavigateToItemMsg struct {
	Item map[string]any
	Key  table.Key
}

// NavigateBackMsg tells the app to go back to previous view
type NavigateBackMsg struct{}

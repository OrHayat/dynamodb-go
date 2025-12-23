package dynamo

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/orhayat/dynamodb-go/table"

	tea "github.com/charmbracelet/bubbletea"
)

// Client wraps the DynamoDB client with TUI-friendly async methods
type Client struct {
	db     *dynamodb.Client
	client *table.Client
}

// NewClient creates a new TUI DynamoDB client
func NewClient(cfg aws.Config) (*Client, error) {
	db := dynamodb.NewFromConfig(cfg)
	client, err := table.NewClient(db, nil, nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		db:     db,
		client: client,
	}, nil
}

// GetTableClient returns the underlying table.Client for direct operations
func (c *Client) GetTableClient() *table.Client {
	return c.client
}

// ListTablesCmd returns a tea.Cmd that fetches all table names
func (c *Client) ListTablesCmd() tea.Cmd {
	return func() tea.Msg {
		var tables []string
		var lastTable *string

		for {
			resp, err := c.db.ListTables(context.Background(), &dynamodb.ListTablesInput{
				ExclusiveStartTableName: lastTable,
			})
			if err != nil {
				return ErrorMsg{Err: err, Operation: "ListTables"}
			}
			tables = append(tables, resp.TableNames...)
			if resp.LastEvaluatedTableName == nil {
				break
			}
			lastTable = resp.LastEvaluatedTableName
		}
		return TablesListMsg{Tables: tables}
	}
}

// DescribeTableCmd returns a tea.Cmd that fetches table schema
func (c *Client) DescribeTableCmd(tableName string) tea.Cmd {
	return func() tea.Msg {
		schema, desc, err := table.DescribeTable(context.Background(), c.client, tableName)
		if err != nil {
			return ErrorMsg{Err: err, Operation: "DescribeTable", Table: tableName}
		}
		return TableSchemaMsg{
			TableName:   tableName,
			Schema:      schema,
			Description: desc,
		}
	}
}

// ScanCmd returns a tea.Cmd that scans items from a table
func (c *Client) ScanCmd(tableSchema *table.TableDefinition, input table.ScanInput) tea.Cmd {
	return func() tea.Msg {
		var items []map[string]any
		nextPage, err := table.Scan(context.Background(), c.client, tableSchema, input, &items)
		if err != nil {
			return ErrorMsg{Err: err, Operation: "Scan", Table: tableSchema.Name}
		}

		return ItemsLoadedMsg{
			Items:       items,
			NextPage:    nextPage,
			HasNextPage: nextPage.HasMore(),
			IsQuery:     false,
		}
	}
}

// QueryCmd returns a tea.Cmd that queries items from a table
func (c *Client) QueryCmd(tableSchema *table.TableDefinition, input table.QueryInput) tea.Cmd {
	return func() tea.Msg {
		var items []map[string]any
		nextPage, err := table.Query(context.Background(), c.client, tableSchema, input, &items)
		if err != nil {
			return ErrorMsg{Err: err, Operation: "Query", Table: tableSchema.Name}
		}

		return ItemsLoadedMsg{
			Items:       items,
			NextPage:    nextPage,
			HasNextPage: nextPage.HasMore(),
			IsQuery:     true,
		}
	}
}

// GetItemCmd returns a tea.Cmd that fetches a single item
func (c *Client) GetItemCmd(tableSchema *table.TableDefinition, key table.Key) tea.Cmd {
	return func() tea.Msg {
		var item map[string]any
		err := table.GetItem(context.Background(), c.client, tableSchema, table.GetItemInput{Key: key}, &item)
		if err != nil {
			return ErrorMsg{Err: err, Operation: "GetItem", Table: tableSchema.Name}
		}
		return ItemDetailMsg{Item: item}
	}
}

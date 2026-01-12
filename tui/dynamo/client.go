package dynamo

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/orhayat/dynamodb-go/table"

	tea "github.com/charmbracelet/bubbletea"
)

// Client wraps the DynamoDB client with TUI-friendly async methods
type Client struct {
	db     *dynamodb.Client
	client *table.Client
	logger *slog.Logger
}

// NewClient creates a new TUI DynamoDB client
func NewClient(cfg aws.Config, logger *slog.Logger) (*Client, error) {
	db := dynamodb.NewFromConfig(cfg)
	client, err := table.NewClient(db, nil, nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		db:     db,
		client: client,
		logger: logger,
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
		pages := 0

		for {
			resp, err := c.db.ListTables(context.Background(), &dynamodb.ListTablesInput{
				ExclusiveStartTableName: lastTable,
			})
			if err != nil {
				c.logger.Error("ListTables failed", "error", err)
				return ErrorMsg{Err: err, Operation: "ListTables"}
			}
			pages++
			tables = append(tables, resp.TableNames...)
			if resp.LastEvaluatedTableName == nil {
				break
			}
			lastTable = resp.LastEvaluatedTableName
		}
		c.logger.Info("ListTables",
			"op", "list",
			"pages", pages,
			"tables", len(tables),
		)
		return TablesListMsg{Tables: tables}
	}
}

// DescribeTableCmd returns a tea.Cmd that fetches table schema
func (c *Client) DescribeTableCmd(tableName string) tea.Cmd {
	return func() tea.Msg {
		c.logger.Info("DescribeTable", "op", "describe", "table", tableName)
		schema, desc, err := table.DescribeTable(context.Background(), c.client, tableName)
		if err != nil {
			c.logger.Error("DescribeTable failed", "table", tableName, "error", err)
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
		c.logger.Info("Scan",
			"op", "scan",
			"table", tableSchema.Name,
			"index", input.Index,
			"hasStartKey", input.PaginationKey.HasMore(),
		)
		var items []map[string]any
		nextPage, err := table.Scan(context.Background(), c.client, tableSchema, input, &items)
		if err != nil {
			c.logger.Error("Scan failed", "error", err)
			return ErrorMsg{Err: err, Operation: "Scan", Table: tableSchema.Name}
		}

		c.logger.Info("Scan result",
			"op", "scan",
			"table", tableSchema.Name,
			"index", input.Index,
			"items", len(items),
			"hasMore", nextPage.HasMore(),
		)

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
		c.logger.Info("Query",
			"op", "query",
			"table", tableSchema.Name,
			"index", input.Index,
			"pk", input.Key.PK,
			"sk", input.Key.SK,
			"hasStartKey", input.PaginationKey.HasMore(),
		)
		var items []map[string]any
		nextPage, err := table.Query(context.Background(), c.client, tableSchema, input, &items)
		if err != nil {
			c.logger.Error("Query failed", "error", err)
			return ErrorMsg{Err: err, Operation: "Query", Table: tableSchema.Name}
		}

		c.logger.Info("Query result",
			"op", "query",
			"table", tableSchema.Name,
			"index", input.Index,
			"pk", input.Key.PK,
			"items", len(items),
			"hasMore", nextPage.HasMore(),
		)

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

// ScanSync performs a synchronous scan and returns items directly
func (c *Client) ScanSync(tableSchema *table.TableDefinition, input table.ScanInput) ([]map[string]any, table.PaginationKey, bool, error) {
	var items []map[string]any
	nextPage, err := table.Scan(context.Background(), c.client, tableSchema, input, &items)
	if err != nil {
		return nil, table.PaginationKey{}, false, err
	}
	return items, nextPage, nextPage.HasMore(), nil
}

// QuerySync performs a synchronous query and returns items directly
func (c *Client) QuerySync(tableSchema *table.TableDefinition, input table.QueryInput) ([]map[string]any, table.PaginationKey, bool, error) {
	var items []map[string]any
	nextPage, err := table.Query(context.Background(), c.client, tableSchema, input, &items)
	if err != nil {
		return nil, table.PaginationKey{}, false, err
	}
	return items, nextPage, nextPage.HasMore(), nil
}

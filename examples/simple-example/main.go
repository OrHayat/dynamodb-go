package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"reflect"
	"slices"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/orhayat/dynamodb-go/table"
)

type logKey int

const slogKey logKey = 1

type animalTablesCli struct {
	LoggerLevel     string                `help:"log level" default:"info" enum:"debug,info,warn,error"`
	Endpoint        string                `help:"dynamodb endpoint url, useful for local testing with dynamodb local or localstack" default:""`
	CreateTable     CreateAnimalTables    `cmd:"" help:"create the animals table"`
	DeleteTable     DeleteAnimalTable     `cmd:"" help:"delete the animals table"`
	PutItem         PutAnimalTableItem    `cmd:"" help:"put item to the example table"`
	BatchWriteItems BatchLockItems        `cmd:"" help:"batch put items to the example table"`
	BatchGetItems   BatchReadLocks        `cmd:"" help:"batch get items from the example table"`
	GetItem         GetAnimalTable        `cmd:"" help:"get item from the example table"`
	DeleteItem      DeleteAnimalTableItem `cmd:"" help:"delete item from the example table"`
}

func (cli *animalTablesCli) AfterApply(ctx *kong.Context) error {
	// set up logger
	var logLevel slog.Leveler
	switch cli.LoggerLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		return fmt.Errorf("unknown log level: %s", cli.LoggerLevel)
	}
	// slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	h := slog.NewJSONHandler(os.Stderr,
		&slog.HandlerOptions{
			Level:       logLevel,
			AddSource:   false,
			ReplaceAttr: nil,
		},
	)
	logger := slog.New(h)
	ctx.Bind(logger)
	// ctx.Bind.(*context.Context) = context.WithValue(*ctx.Bind.(*context.Context), slogKey, logger)
	return nil
}

type AnimalTableSeclector struct {
	AnimalType string `help:"partition key value" required:""`
	Name       string `help:"sort key value" required:""`
}

type CreateAnimalTables struct{}

func (cli *CreateAnimalTables) Run(ctx context.Context, logger *slog.Logger, client *table.Client) (err error) {
	logger.Info("Creating animal table")
	err = table.CreateTable(ctx, client, &s_animalTable)
	if err != nil {
		return err
	}
	err = table.CreateTable(ctx, client, &s_locksTable)
	return err
}

type DeleteAnimalTable struct{}

func (cli *DeleteAnimalTable) Run(ctx context.Context, logger *slog.Logger, client *table.Client) (err error) {
	logger.Info("Deleting animal table")
	err = table.DeleteTable(ctx, client, &s_animalTable)
	if err != nil {
		return err
	}
	err = table.DeleteTable(ctx, client, &s_locksTable)
	return err
}

type GetAnimalTable struct {
	AnimalTableSeclector
}

type AnimalBase struct {
	AnimalType string `dynamodbav:"AnimalType"` // PK
	AnimalName string `dynamodbav:"AnimalName"` // SK
	Age        int    `dynamodbav:"Age"`
}

type Dog struct {
	AnimalBase
	Breed       string `dynamodbav:"Breed"`
	Trained     bool   `dynamodbav:"Trained"`
	FavoriteToy string `dynamodbav:"FavoriteToy"`
}
type Cat struct {
	AnimalBase
	Breed          string `dynamodbav:"Breed"`
	Indoor         bool   `dynamodbav:"Indoor"`
	ClimbsCurtains bool   `dynamodbav:"ClimbsCurtains"`
}

type Turtle struct {
	AnimalBase
	Species       string `dynamodbav:"Species"`
	ShellLengthCm int    `dynamodbav:"ShellLengthCm"`
	Aquatic       bool   `dynamodbav:"Aquatic"`
}

type Rabbit struct {
	AnimalBase
	Breed        string `dynamodbav:"Breed"`
	FurColor     string `dynamodbav:"FurColor"`
	FavoriteFood string `dynamodbav:"FavoriteFood"`
	HopsPerMin   int    `dynamodbav:"HopsPerMin"`
	Indoor       bool   `dynamodbav:"Indoor"`
}

type GenericAnimal struct {
	AnimalBase
	Attributes map[string]any `dynamodbav:"Attributes"`
}

type schemaRegistry struct {
	registeredTypes map[string]func() any
}

func (r schemaRegistry) RegisterType(key string, schema any) (err error) {
	typ := reflect.TypeOf(schema)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct type, got %s", typ.Kind())
	}
	r.registeredTypes[key] = func() any {
		return reflect.New(typ).Interface()
	}
	return nil
}

func (r schemaRegistry) GetScehma(k string) any {
	cb, ok := r.registeredTypes[k]
	if ok {
		return cb()
	}
	return GenericAnimal{}
}

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

var s_schemaRegistry = schemaRegistry{
	registeredTypes: map[string]func() any{},
}

func init() {
	Must(s_schemaRegistry.RegisterType("cat", Cat{}))
	Must(s_schemaRegistry.RegisterType("dog", Dog{}))
	Must(s_schemaRegistry.RegisterType("turtle", Turtle{}))
	Must(s_schemaRegistry.RegisterType("rabbit", Rabbit{}))
}
func generateString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}

type KeyValue struct {
	Key string
	Val any
}

func simpleFuzzStruct(val any, predefinedFields []KeyValue) (err error) {
	v := reflect.ValueOf(val)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return fmt.Errorf("fuzz input have to be non nil pointer")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("fuzz input have to be struct pointer")
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fv := v.Field(i)
		if !fv.CanSet() {
			continue
		}
		if index := slices.IndexFunc(predefinedFields, func(fv KeyValue) bool { return fv.Key == f.Name }); index != -1 {
			fv.Set(reflect.ValueOf(predefinedFields[index].Val))
			continue
		}
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(generateString(15))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fv.SetInt(rand.Int64())
		case reflect.Bool:
			r := rand.Int()
			switch r % 3 {
			case 0:
				fv.SetBool(false)
			case 1:
				fv.SetBool(true)
			case 2:
				// leave it as is incase someone else filled it
			}
		case reflect.Float32, reflect.Float64:
			fv.SetFloat(rand.Float64() * 1000.0)
		case reflect.Struct:
			err = simpleFuzzStruct(fv.Addr().Interface(), predefinedFields)
			if err != nil {
				return err
			}
		case reflect.Slice | reflect.Array:
			for i := 0; i < fv.Len(); i++ {
				err = simpleFuzzStruct(fv.Index(i).Addr().Interface(), predefinedFields)
				if err != nil {
					return err
				}
			}
		case reflect.Map:
			fv.MapRange()
			for iter := fv.MapRange(); iter.Next(); {
				v := iter.Value()
				err = simpleFuzzStruct(v.Addr().Interface(), predefinedFields)
				if err != nil {
					return err
				}
			}
		default:
			// skip unsupported types
			continue
		}
	}
	return nil
}

func (cli *GetAnimalTable) Run(ctx context.Context, logger *slog.Logger, client *table.Client) (err error) {
	logger.Info("Getting animal table item", "Type", cli.AnimalType, "Name", cli.Name)
	typ := strings.ToLower(cli.AnimalType)
	res := s_schemaRegistry.GetScehma(typ)
	key := table.Key{PK: cli.AnimalType, SK: cli.Name}
	err = table.GetItem(ctx, client, &s_animalTable, key, &res)
	if err != nil {
		return fmt.Errorf("failed to get item from schema:%w", err)
	}
	logger.InfoContext(ctx, "fetched animal from db as generic animal", "animal", res)
	asGeneric, err := table.GetItemOf[GenericAnimal](ctx, client, &s_animalTable, key)
	if err != nil {
		return fmt.Errorf("failed to get item of GenericAnimal:%w", err)
	}
	logger.InfoContext(ctx, "fetched animal from db as generic animal", "animal", asGeneric)
	asJson, err := table.GetAsJSON(ctx, client, &s_animalTable, key)
	if err != nil {
		return fmt.Errorf("failed to get item as any type:%w", err)
	}
	logger.InfoContext(ctx, "fetched animal from db as any type", "animal", asJson)
	return nil
}

type DeleteAnimalTableItem struct {
	AnimalTableSeclector
}

func (cli *DeleteAnimalTableItem) Run(ctx context.Context, logger *slog.Logger, client *table.Client) error {
	key := table.Key{PK: cli.AnimalType, SK: cli.Name}
	err := table.DeleteItem(ctx, client, &s_animalTable, key)
	if err != nil {
		return err
	}
	logger.InfoContext(ctx, "deleted animal from db", "Type", cli.AnimalType, "Name", cli.Name)
	return nil
}

type BatchReadLocks struct {
	Items []string `arg:"" help:"list of items to read"`
}

func (cli *BatchReadLocks) Run(ctx context.Context, logger *slog.Logger, client *table.Client) (err error) {
	if len(cli.Items) == 0 {
		return fmt.Errorf("no items to read")
	}
	keys := []table.Key{}
	for _, item := range cli.Items {
		keys = append(keys, table.Key{PK: item})
	}
	res, err := table.BatchGetItemsFromSingleTable[lock](ctx, client, &s_locksTable, keys)
	if err != nil {
		return fmt.Errorf("failed to batch get items from single table: %w", err)
	}
	logger.InfoContext(ctx, "fetched locks from db", "locks", res)
	return nil
}

type BatchLockItems struct {
	Items  []string `arg:"" help:"list of items to lock"`
	Delete bool
}
type lock struct {
	LockID string `dynamodbav:"LockID"`
	Owner  string `dynamodbav:"Owner"`
	TTL    int64  `dynamodbav:"TTL"`
	Info   string `dynamodbav:"Info"`
}

func (cli *BatchLockItems) Run(ctx context.Context, logger *slog.Logger, client *table.Client) (err error) {
	if len(cli.Items) == 0 {
		return fmt.Errorf("no items to lock")
	}
	putRequests := []table.PutRequest{}
	deleteRequests := []table.DeleteRequest{}

	if !cli.Delete {
		for _, item := range cli.Items {
			putRequests = append(putRequests, table.PutRequest{
				Item: lock{
					LockID: item,
					Owner:  "cli",
					TTL:    1700000000,
					Info:   fmt.Sprintf("lock for item %s", item),
				},
			})
		}
	} else {
		for _, item := range cli.Items {
			deleteRequests = append(deleteRequests, table.DeleteRequest{
				Key: table.Key{PK: item},
			})
		}
	}
	err = table.BatchWriteItems(ctx, client, []table.WriteRequest{
		{
			Table:       &s_locksTable,
			PutRequests: putRequests,
			DeleteItems: deleteRequests,
		},
	})
	return err
}

type PutAnimalTableItem struct {
	AnimalTableSeclector
	Upsert bool `help:"if set will overwrite existing item with same PK/SK" default:"false"`
}

func (cli *PutAnimalTableItem) Run(ctx context.Context, logger *slog.Logger, client *table.Client) error {
	entry := s_schemaRegistry.GetScehma(cli.AnimalType)
	logger = logger.With("Type", cli.AnimalType, "Name", cli.Name)
	err := simpleFuzzStruct(entry, []KeyValue{
		{s_animalTable.PrimaryKey.Name, cli.AnimalType},
		{s_animalTable.RangeKey.Name, cli.Name},
	})
	logger.InfoContext(ctx, "Putting animal to db", "entry", entry)
	if err != nil {
		return fmt.Errorf("put item:%w", err)
	}
	err = table.PutItem(ctx, client, &s_animalTable, entry, table.WithUpsert(cli.Upsert))
	if err != nil {
		return err
	}
	logger.InfoContext(ctx, "inserted animal to db", entry)
	return nil
}

var s_animalTable = table.TableDefinition{
	Name: "Animals",
	PrimaryKey: table.AttributeDefinition{
		Name: "AnimalType",
		Type: types.ScalarAttributeTypeS,
	},
	RangeKey: table.AttributeDefinition{
		Name: "AnimalName",
		Type: types.ScalarAttributeTypeS,
	},
}

var s_locksTable = table.TableDefinition{
	Name: "Locks",
	PrimaryKey: table.AttributeDefinition{
		Name: "LockID",
		Type: types.ScalarAttributeTypeS,
	},
}

func main() {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to load SDK config, %v", err)
		os.Exit(1)
	}
	dynamoClient := dynamodb.NewFromConfig(cfg,
		func(o *dynamodb.Options) {
			o.Credentials = credentials.NewStaticCredentialsProvider("local", "local", "local")
			o.Region = "us-east-1"
			o.BaseEndpoint = aws.String("http://localhost:8001")
		})
	tablesClient, err := table.NewClient(dynamoClient, nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to create table client, %v", err)
		os.Exit(1)
	}
	parsedCli := kong.Parse(&animalTablesCli{},
		kong.BindTo(ctx, (*context.Context)(nil)),
		kong.Bind(cfg),
		kong.Bind(tablesClient),
	)
	err = parsedCli.Run()
	parsedCli.FatalIfErrorf(err)
}

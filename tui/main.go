package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/orhayat/dynamodb-go/tui/app"
	"github.com/orhayat/dynamodb-go/tui/dynamo"
)

var (
	profileFlag = flag.String("profile", "", "AWS profile to use (default: uses AWS_PROFILE env var or 'default')")
	regionFlag  = flag.String("region", "", "AWS region (default: uses AWS_REGION env var or profile's region)")
)

func main() {
	flag.Parse()

	// Determine profile
	profile := *profileFlag
	if profile == "" {
		profile = os.Getenv("AWS_PROFILE")
	}
	if profile == "" {
		profile = "default"
	}

	// Load AWS config
	ctx := context.Background()
	var opts []func(*config.LoadOptions) error

	if *profileFlag != "" {
		opts = append(opts, config.WithSharedConfigProfile(*profileFlag))
	}
	if *regionFlag != "" {
		opts = append(opts, config.WithRegion(*regionFlag))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading AWS config: %v\n", err)
		os.Exit(1)
	}

	// Determine region for display
	region := cfg.Region
	if region == "" {
		region = "unknown"
	}

	// Setup logger
	logFile, err := os.OpenFile("/tmp/tui-debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create DynamoDB client with logger
	client, err := dynamo.NewClient(cfg, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating DynamoDB client: %v\n", err)
		os.Exit(1)
	}

	// Create and run app
	appModel := app.New(client, logger, profile, region)
	p := tea.NewProgram(appModel, tea.WithAltScreen(), tea.WithoutSignalHandler())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

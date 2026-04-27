package main

import (
	"bucket-brigade/api"
	"bucket-brigade/config"
	"bucket-brigade/dbs"
	"bucket-brigade/pkg/buckets"
	"bucket-brigade/pkg/contents"
	"bucket-brigade/pkg/objects"
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

func main() {
	// Define flags
	addrFlag := flag.String("addr", "", "Address to listen on (e.g. :8080)")
	logLevelFlag := flag.String("loglevel", "info", "Log level (debug, info, warn, error, fatal, panic)")
	flag.Parse()

	// Setup logging
	level, err := logrus.ParseLevel(*logLevelFlag)
	if err != nil {
		level = logrus.InfoLevel
	}
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(level)

	// Load configuration from properties.yaml
	cfg, err := config.LoadConfig("properties")
	if err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}

	// Use flag address if provided, else use config
	addr := *addrFlag
	if addr == "" {
		addr = fmt.Sprintf(":%d", cfg.Server.Port)
	}

	logrus.Infof("Hello, Bucket Brigade!")
	logrus.Infof("SQLite Path: %s", cfg.Database.SQLitePath)
	logrus.Infof("Server Address: %s", addr)

	// Initialize database
	db, err := dbs.InitDb(cfg)
	if err != nil {
		logrus.Fatalf("Failed to initialize database: %v", err)
	}

	// Run migrations
	if cfg.Database.Migrations.AutoRun {
		if err := dbs.Migrate(cfg, db); err != nil {
			logrus.Fatalf("Failed to run migrations: %v", err)
		}
	} else {
		logrus.Info("Automatic database migrations are disabled")
	}

	logrus.Info("Database initialized and ready!")

	// Dependency Injection
	bucketRepo := buckets.NewBucketRepository(db)
	contentRepo := contents.NewObjectContentRepository(db)
	objectRepo := objects.NewObjectRepository(db)

	bucketService := buckets.NewBucketService(bucketRepo)
	contentService := contents.NewObjectContentService(contentRepo, cfg)
	objectService := objects.NewObjectService(objectRepo, cfg, bucketService, contentService)
	contentService.CleanupZeroRefContents()

	// Instantiate and start REST API
	restApi := api.NewRESTApiV1(cfg, objectService)
	logrus.Infof("Starting server on %s", addr)
	if err := restApi.Start(addr); err != nil {
		logrus.Fatalf("Failed to start server: %v", err)
	}
}

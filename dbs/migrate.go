package dbs

import (
	"bucket-brigade/config"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func Migrate(cfg *config.Config, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	migrationsPath, err := resolveMigrationsPath(cfg.Database.Migrations.Path)
	if err != nil {
		return err
	}

	driver, err := migratesqlite.WithInstance(sqlDB, &migratesqlite.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsPath, "sqlite3", driver)
	if err != nil {
		return err
	}

	logrus.Infof("Running database migrations from %s", migrationsPath)
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	logrus.Info("Database migrations completed successfully")
	return nil
}

func resolveMigrationsPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(absPath); err == nil {
		return absPath, nil
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve migrations path: runtime caller unavailable")
	}

	repoRelativePath := filepath.Join(filepath.Dir(filepath.Dir(currentFile)), path)
	absRepoRelativePath, err := filepath.Abs(repoRelativePath)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(absRepoRelativePath); err != nil {
		return "", fmt.Errorf("resolve migrations path %q: %w", path, err)
	}

	return absRepoRelativePath, nil
}

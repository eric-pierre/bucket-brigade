package dbs

import (
	"bucket-brigade/config"
	"errors"
	"fmt"
	"strings"

	"github.com/mattn/go-sqlite3"
	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func sqliteDSN(cfg *config.Config) string {
	path := cfg.Database.SQLitePath
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}

	return fmt.Sprintf(
		"%s%s_journal_mode=%s&_busy_timeout=%d&_foreign_keys=on",
		path,
		separator,
		cfg.Database.SQLite.JournalMode,
		cfg.Database.SQLite.BusyTimeout,
	)
}

func InitDb(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(sqliteDSN(cfg)), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.Use(otelgorm.NewPlugin()); err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(cfg.Database.SQLite.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.SQLite.MaxIdleConns)
	return db, nil
}

func IsUniqueConstraintError(err error) bool {
	var sqliteErr sqlite3.Error
	return errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
}

type HealthService struct{ db *gorm.DB }

func NewHealthService(db *gorm.DB) *HealthService {
	return &HealthService{db: db}
}

func (h *HealthService) Ping() error {
	sqlDB, err := h.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

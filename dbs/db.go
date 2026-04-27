package dbs

import (
	"bucket-brigade/config"
	"errors"
	"fmt"
	"strings"

	"github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
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
		"%s%s_journal_mode=%s&_busy_timeout=%d",
		path,
		separator,
		cfg.Database.SQLite.JournalMode,
		cfg.Database.SQLite.BusyTimeout,
	)
}

func InitDb(cfg *config.Config) (*gorm.DB, error) {
	logrus.Infof("Initializing database with path: %s", cfg.Database.SQLitePath)
	db, err := gorm.Open(sqlite.Open(sqliteDSN(cfg)), &gorm.Config{})
	if err != nil {
		logrus.Errorf("Failed to open database: %v", err)
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		logrus.Errorf("Failed to get sql.DB handle: %v", err)
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

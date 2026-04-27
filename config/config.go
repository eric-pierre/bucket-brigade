package config

import (
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

type Config struct {
	Database struct {
		SQLitePath string `mapstructure:"sqlite-path"`
		SQLite     struct {
			JournalMode  string `mapstructure:"journal-mode"`
			BusyTimeout  int    `mapstructure:"busy-timeout-ms"`
			MaxOpenConns int    `mapstructure:"max-open-conns"`
			MaxIdleConns int    `mapstructure:"max-idle-conns"`
		} `mapstructure:"sqlite"`
		Migrations struct {
			AutoRun bool   `mapstructure:"auto-run"`
			Path    string `mapstructure:"path"`
		} `mapstructure:"migrations"`
	} `mapstructure:"database"`
	Server struct {
		Port           int   `mapstructure:"port"`
		MaxUploadBytes int64 `mapstructure:"max-upload-bytes"`
		Timeouts       struct {
			Read  time.Duration `mapstructure:"read"`
			Write time.Duration `mapstructure:"write"`
			Idle  time.Duration `mapstructure:"idle"`
		} `mapstructure:"timeouts"`
		Validation struct {
			ObjectRouteParamMaxLength int `mapstructure:"object-route-param-max-length"`
		} `mapstructure:"validation"`
	} `mapstructure:"server"`
	Storage struct {
		BasePath string `mapstructure:"base-path"`
	} `mapstructure:"storage"`
}

var appConfig *Config

// LoadConfig reads the specified config file and maps it to the Config struct
func LoadConfig(configName string) (*Config, error) {
	if configName == "" {
		configName = "properties"
	}

	viper.Reset() // Reset viper to avoid interference between calls
	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetDefault("database.sqlite-path", "bucket_brigade.db")
	viper.SetDefault("database.sqlite.journal-mode", "WAL")
	viper.SetDefault("database.sqlite.busy-timeout-ms", 5000)
	viper.SetDefault("database.sqlite.max-open-conns", 1)
	viper.SetDefault("database.sqlite.max-idle-conns", 1)
	viper.SetDefault("database.migrations.auto-run", true)
	viper.SetDefault("database.migrations.path", "db/migrations")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.max-upload-bytes", int64(5<<30))
	viper.SetDefault("server.timeouts.read", "30s")
	viper.SetDefault("server.timeouts.write", "60s")
	viper.SetDefault("server.timeouts.idle", "120s")
	viper.SetDefault("server.validation.object-route-param-max-length", 255)
	viper.SetDefault("storage.base-path", "data")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	config := &Config{}
	if err := viper.Unmarshal(config, func(dc *mapstructure.DecoderConfig) {
		dc.DecodeHook = mapstructure.StringToTimeDurationHookFunc()
	}); err != nil {
		return nil, err
	}

	return config, nil
}

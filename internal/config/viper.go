package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// EnvPrefix namespaces environment variable overrides: "api_key" is
// overridden by APP_API_KEY.
const EnvPrefix = "APP"

// NewViper builds the raw configuration source.
func NewViper() (*viper.Viper, error) {
	v := viper.New()
	setDefaults(v)

	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if file := os.Getenv("CONFIG_FILE"); file != "" {
		v.SetConfigFile(file)
	} else {
		v.SetConfigName(envOrDefault("CONFIG_NAME", "config"))
		v.SetConfigType(envOrDefault("CONFIG_TYPE", "yml"))
		if path := os.Getenv("CONFIG_PATH"); path != "" {
			v.AddConfigPath(path)
		}
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/app")
		v.AddConfigPath("..")
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errorsAs(err, &notFound) {
			return nil, fmt.Errorf("read configuration file: %w", err)
		}
	}

	return v, nil
}

// Init loads and validates the configuration in one step.
func Init() (*viper.Viper, *Config, error) {
	v, err := NewViper()
	if err != nil {
		return nil, nil, err
	}
	cfg, err := Load(v)
	if err != nil {
		return nil, nil, err
	}
	return v, cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func errorsAs(err error, target *viper.ConfigFileNotFoundError) bool {
	notFound, ok := err.(viper.ConfigFileNotFoundError)
	if ok {
		*target = notFound
	}
	return ok
}

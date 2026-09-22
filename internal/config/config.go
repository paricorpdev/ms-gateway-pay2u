package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Environment names understood by the application.
const (
	EnvDevelopment = "development"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// Config is the fully typed, validated application configuration.
type Config struct {
	App      AppConfig
	Web      WebConfig
	Log      LogConfig
	Database DatabaseConfig
	Pay2U    Pay2UConfig
	APIKey   string
}

type AppConfig struct {
	Name            string
	Env             string
	Timezone        string
	ShutdownTimeout time.Duration
}

func (a AppConfig) IsProduction() bool { return a.Env == EnvProduction }

type WebConfig struct {
	Host                    string
	Port                    int
	Prefork                 bool
	BodyLimit               int
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	IdleTimeout             time.Duration
	RequestTimeout          time.Duration
	TrustedProxies          []string
	EnableTrustedProxyCheck bool
	CORS                    CORSConfig
	RateLimit               RateLimitConfig
	Metrics                 MetricsConfig
}

func (w WebConfig) Address() string { return fmt.Sprintf("%s:%d", w.Host, w.Port) }

type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
}

type RateLimitConfig struct {
	Enabled    bool
	Max        int
	Expiration time.Duration
}

type MetricsConfig struct {
	Enabled bool
	Path    string
}

type LogConfig struct {
	Level  string
	Format string
}

type DatabaseConfig struct {
	Postgres PostgresConfig
	Redis    RedisConfig
}

type PostgresConfig struct {
	Host            string
	Port            int
	Username        string
	Password        string
	Name            string
	SSLMode         string
	Timezone        string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	LogLevel        string
	SlowThreshold   time.Duration
}

func (p PostgresConfig) DSN() string {
	pairs := [][2]string{
		{"TimeZone", p.Timezone},
		{"host", p.Host},
		{"port", strconv.Itoa(p.Port)},
		{"user", p.Username},
		{"password", p.Password},
		{"dbname", p.Name},
		{"sslmode", p.SSLMode},
	}

	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, pair[0]+"="+dsnValue(pair[1]))
	}
	return strings.Join(parts, " ")
}

func dsnValue(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\n\r\v\f'\\") {
		return value
	}
	escaped := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(value)
	return "'" + escaped + "'"
}

func (p PostgresConfig) Redacted() string {
	return fmt.Sprintf("%s:%d/%s?sslmode=%s", p.Host, p.Port, p.Name, p.SSLMode)
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	Database int
	TLS      bool
	PoolSize int
}

func (r RedisConfig) Redacted() string { return fmt.Sprintf("%s:%d/%d", r.Host, r.Port, r.Database) }

type Pay2UConfig struct {
	Sandbox           bool
	BaseURLSandbox    string
	BaseURLProduction string
	OAuthURL          string
	TokenURL          string
	BillingVAURL      string
	BillingQRISURL    string
	BillingCCURL      string
	BillingGetURL     string
	ClientID          string
	ClientSecret      string
	MerchantCode      string
	MerchantUser      string
	MerchantPass      string
	MerchantDomain    string
	Timeout           time.Duration
	OAuthTokenTTL     time.Duration
	CallbackBaseURL   string
}

func (p Pay2UConfig) BaseURL() string {
	if p.Sandbox {
		return p.BaseURLSandbox
	}
	return p.BaseURLProduction
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "paygate")
	v.SetDefault("app.env", EnvDevelopment)
	v.SetDefault("app.timezone", "Asia/Jakarta")
	v.SetDefault("app.shutdown_timeout", "15s")

	v.SetDefault("web.host", "0.0.0.0")
	v.SetDefault("web.port", 8080)
	v.SetDefault("web.prefork", false)
	v.SetDefault("web.body_limit", 8*1024*1024)
	v.SetDefault("web.read_timeout", "15s")
	v.SetDefault("web.write_timeout", "30s")
	v.SetDefault("web.idle_timeout", "75s")
	v.SetDefault("web.request_timeout", "30s")
	v.SetDefault("web.trusted_proxies", []string{})
	v.SetDefault("web.enable_trusted_proxy_check", false)

	v.SetDefault("web.cors.allow_origins", []string{"http://localhost", "http://127.0.0.1"})
	v.SetDefault("web.cors.allow_methods", []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	v.SetDefault("web.cors.allow_headers", []string{"Origin", "Content-Type", "Accept", "Content-Length", "Accept-Encoding", "Authorization", "X-Request-Id", "X-API-Key"})
	v.SetDefault("web.cors.expose_headers", []string{"Content-Disposition", "X-Request-Id"})
	v.SetDefault("web.cors.allow_credentials", true)
	v.SetDefault("web.cors.max_age", 3600)

	v.SetDefault("web.rate_limit.enabled", true)
	v.SetDefault("web.rate_limit.max", 120)
	v.SetDefault("web.rate_limit.expiration", "1m")

	v.SetDefault("web.metrics.enabled", false)
	v.SetDefault("web.metrics.path", "/metrics")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	v.SetDefault("database.postgresql.host", "localhost")
	v.SetDefault("database.postgresql.port", 5432)
	v.SetDefault("database.postgresql.sslmode", "disable")
	v.SetDefault("database.postgresql.timezone", "Asia/Jakarta")
	v.SetDefault("database.postgresql.pool.idle", 10)
	v.SetDefault("database.postgresql.pool.max", 100)
	v.SetDefault("database.postgresql.pool.lifetime", "5m")
	v.SetDefault("database.postgresql.pool.idle_time", "5m")
	v.SetDefault("database.postgresql.log_level", "warn")
	v.SetDefault("database.postgresql.slow_threshold", "500ms")

	v.SetDefault("database.redis.host", "localhost")
	v.SetDefault("database.redis.port", 6379)
	v.SetDefault("database.redis.database", 0)
	v.SetDefault("database.redis.tls", false)
	v.SetDefault("database.redis.pool_size", 10)

	v.SetDefault("api_key", "dev-api-key-change-me")

	v.SetDefault("pay2u.sandbox", true)
	v.SetDefault("pay2u.base_url_sandbox", "https://api-dev.pay2u.co.id")
	v.SetDefault("pay2u.base_url_production", "https://api.pay2u.co.id")
	v.SetDefault("pay2u.oauth_url", "/security/auth/oauth/token")
	v.SetDefault("pay2u.token_url", "/service/core/trx/token/create")
	v.SetDefault("pay2u.billing_va_url", "/service/core/trx/billing/va/submit")
	v.SetDefault("pay2u.billing_qris_url", "/service/core/trx/billing/qris/submit")
	v.SetDefault("pay2u.billing_cc_url", "/service/core/trx/billing/cc/submit")
	v.SetDefault("pay2u.billing_get_url", "/service/core/trx/billing/get")
	v.SetDefault("pay2u.timeout", "30s")
	v.SetDefault("pay2u.oauth_token_ttl", "50m")
}

func Load(v *viper.Viper) (*Config, error) {
	cfg := &Config{
		App: AppConfig{
			Name:            v.GetString("app.name"),
			Env:             strings.ToLower(v.GetString("app.env")),
			Timezone:        v.GetString("app.timezone"),
			ShutdownTimeout: v.GetDuration("app.shutdown_timeout"),
		},
		Web: WebConfig{
			Host:                    v.GetString("web.host"),
			Port:                    v.GetInt("web.port"),
			Prefork:                 v.GetBool("web.prefork"),
			BodyLimit:               v.GetInt("web.body_limit"),
			ReadTimeout:             v.GetDuration("web.read_timeout"),
			WriteTimeout:            v.GetDuration("web.write_timeout"),
			IdleTimeout:             v.GetDuration("web.idle_timeout"),
			RequestTimeout:          v.GetDuration("web.request_timeout"),
			TrustedProxies:          v.GetStringSlice("web.trusted_proxies"),
			EnableTrustedProxyCheck: v.GetBool("web.enable_trusted_proxy_check"),
			CORS: CORSConfig{
				AllowOrigins:     v.GetStringSlice("web.cors.allow_origins"),
				AllowMethods:     v.GetStringSlice("web.cors.allow_methods"),
				AllowHeaders:     v.GetStringSlice("web.cors.allow_headers"),
				ExposeHeaders:    v.GetStringSlice("web.cors.expose_headers"),
				AllowCredentials: v.GetBool("web.cors.allow_credentials"),
				MaxAge:           v.GetInt("web.cors.max_age"),
			},
			RateLimit: RateLimitConfig{
				Enabled:    v.GetBool("web.rate_limit.enabled"),
				Max:        v.GetInt("web.rate_limit.max"),
				Expiration: v.GetDuration("web.rate_limit.expiration"),
			},
			Metrics: MetricsConfig{
				Enabled: v.GetBool("web.metrics.enabled"),
				Path:    v.GetString("web.metrics.path"),
			},
		},
		Log: LogConfig{
			Level:  strings.ToLower(v.GetString("log.level")),
			Format: strings.ToLower(v.GetString("log.format")),
		},
		Database: DatabaseConfig{
			Postgres: PostgresConfig{
				Host:            v.GetString("database.postgresql.host"),
				Port:            v.GetInt("database.postgresql.port"),
				Username:        v.GetString("database.postgresql.username"),
				Password:        v.GetString("database.postgresql.password"),
				Name:            v.GetString("database.postgresql.name"),
				SSLMode:         v.GetString("database.postgresql.sslmode"),
				Timezone:        v.GetString("database.postgresql.timezone"),
				MaxIdleConns:    v.GetInt("database.postgresql.pool.idle"),
				MaxOpenConns:    v.GetInt("database.postgresql.pool.max"),
				ConnMaxLifetime: v.GetDuration("database.postgresql.pool.lifetime"),
				ConnMaxIdleTime: v.GetDuration("database.postgresql.pool.idle_time"),
				LogLevel:        strings.ToLower(v.GetString("database.postgresql.log_level")),
				SlowThreshold:   v.GetDuration("database.postgresql.slow_threshold"),
			},
			Redis: RedisConfig{
				Host:     v.GetString("database.redis.host"),
				Port:     v.GetInt("database.redis.port"),
				Password: v.GetString("database.redis.password"),
				Database: v.GetInt("database.redis.database"),
				TLS:      v.GetBool("database.redis.tls"),
				PoolSize: v.GetInt("database.redis.pool_size"),
			},
		},
		Pay2U: Pay2UConfig{
			Sandbox:           v.GetBool("pay2u.sandbox"),
			BaseURLSandbox:    strings.TrimRight(v.GetString("pay2u.base_url_sandbox"), "/"),
			BaseURLProduction: strings.TrimRight(v.GetString("pay2u.base_url_production"), "/"),
			OAuthURL:          v.GetString("pay2u.oauth_url"),
			TokenURL:          v.GetString("pay2u.token_url"),
			BillingVAURL:      v.GetString("pay2u.billing_va_url"),
			BillingQRISURL:    v.GetString("pay2u.billing_qris_url"),
			BillingCCURL:      v.GetString("pay2u.billing_cc_url"),
			BillingGetURL:     v.GetString("pay2u.billing_get_url"),
			ClientID:          v.GetString("pay2u.client_id"),
			ClientSecret:      v.GetString("pay2u.client_secret"),
			MerchantCode:      v.GetString("pay2u.merchant_code"),
			MerchantUser:      v.GetString("pay2u.merchant_user"),
			MerchantPass:      v.GetString("pay2u.merchant_pass"),
			MerchantDomain:    v.GetString("pay2u.merchant_domain"),
			Timeout:           v.GetDuration("pay2u.timeout"),
			OAuthTokenTTL:     v.GetDuration("pay2u.oauth_token_ttl"),
			CallbackBaseURL:   strings.TrimRight(v.GetString("pay2u.callback_base_url"), "/"),
		},
		APIKey: v.GetString("api_key"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	var problems []string

	require := func(value, key string) {
		if strings.TrimSpace(value) == "" {
			problems = append(problems, fmt.Sprintf("%s is required", key))
		}
	}

	switch c.App.Env {
	case EnvDevelopment, EnvStaging, EnvProduction:
	default:
		problems = append(problems, fmt.Sprintf(
			"app.env must be one of %q, %q or %q (got %q)",
			EnvDevelopment, EnvStaging, EnvProduction, c.App.Env))
	}

	if c.Web.Port < 1 || c.Web.Port > 65535 {
		problems = append(problems, fmt.Sprintf("web.port must be between 1 and 65535 (got %d)", c.Web.Port))
	}

	require(c.Database.Postgres.Host, "database.postgresql.host")
	require(c.Database.Postgres.Username, "database.postgresql.username")
	require(c.Database.Postgres.Name, "database.postgresql.name")
	switch c.Database.Postgres.SSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		problems = append(problems, fmt.Sprintf(
			"database.postgresql.sslmode %q is not a valid libpq mode", c.Database.Postgres.SSLMode))
	}
	if c.App.IsProduction() && c.Database.Postgres.SSLMode == "disable" {
		problems = append(problems, "database.postgresql.sslmode must not be \"disable\" in production")
	}

	if tz := c.Database.Postgres.Timezone; tz == "" {
		problems = append(problems, "database.postgresql.timezone is required")
	} else if _, err := time.LoadLocation(tz); err != nil {
		problems = append(problems, fmt.Sprintf(
			"database.postgresql.timezone %q is not a known timezone", tz))
	}
	if c.Database.Postgres.MaxOpenConns > 0 && c.Database.Postgres.MaxIdleConns > c.Database.Postgres.MaxOpenConns {
		problems = append(problems, "database.postgresql.pool.idle must not exceed database.postgresql.pool.max")
	}

	require(c.Database.Redis.Host, "database.redis.host")

	if c.App.IsProduction() {
		require(c.APIKey, "api_key")
		require(c.Pay2U.ClientID, "pay2u.client_id")
		require(c.Pay2U.ClientSecret, "pay2u.client_secret")
		require(c.Pay2U.MerchantCode, "pay2u.merchant_code")
		require(c.Pay2U.MerchantUser, "pay2u.merchant_user")
		require(c.Pay2U.MerchantPass, "pay2u.merchant_pass")
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(problems, "\n  - "))
}

var ErrMissingConfigFile = errors.New("configuration file not found")

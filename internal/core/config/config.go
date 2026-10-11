// Package config loads and validates all application configuration from
// environment variables at startup. It is the single source of truth for
// runtime settings — no other package should call os.Getenv directly.
package config

import "github.com/caarlos0/env/v11"

// Config holds every runtime setting the application needs.
// Fields tagged with required:true will cause Load to return an error
// if the variable is absent or empty.
type Config struct {
	// Server
	Port          int `env:"PORT" envDefault:"3001"`
	DashboardPort int `env:"DASHBOARD_PORT" envDefault:"3002"`

	// Database — must use PgBouncer transaction pooler (port 6543) at runtime.
	// Include ?pgbouncer=true&connection_limit=1 in the URL.
	DatabaseURL string `env:"DATABASE_URL,required"`

	// Redis
	RedisHost     string `env:"REDIS_HOST" envDefault:"127.0.0.1"`
	RedisPort     int    `env:"REDIS_PORT" envDefault:"6379"`
	RedisPassword string `env:"REDIS_PASSWORD"`

	// AWS S3
	AWSRegion          string `env:"AWS_REGION,required"`
	AWSAccessKeyID     string `env:"AWS_ACCESS_KEY_ID,required"`
	AWSSecretAccessKey string `env:"AWS_SECRET_ACCESS_KEY,required"`
	S3Bucket           string `env:"AWS_S3_BUCKET,required"`

	// Firebase — base64-encoded service account JSON
	FirebaseServiceAccountBase64 string `env:"FIREBASE_SERVICE_ACCOUNT_BASE64,required"`

	// AI
	GeminiAPIKey string `env:"GEMINI_API_KEY,required"`

	// Auth
	// AdminAPIKey guards reference-data writes and the user listing via the
	// X-Admin-Key header. Empty disables those routes.
	AdminAPIKey string `env:"ADMIN_API_KEY"`

	// App
	DashboardURL  string `env:"DASHBOARD_URL"`
	MailgunAPIKey string `env:"MAILGUN_API_KEY"`
}

// Load parses environment variables into a Config struct.
// It returns an error if any required variable is missing.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

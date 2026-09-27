// Package config loads process configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds runtime configuration for the API server.
type Config struct {
	DatabaseURL        string
	Port               string
	JWTSecret          string
	CORSAllowedOrigins []string
	UploadDir          string
}

// Load reads configuration from the environment and fails fast on missing values.
func Load() (Config, error) {
	var missing []string
	get := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}
	cfg := Config{
		DatabaseURL: get("DATABASE_URL"),
		Port:        strings.TrimSpace(os.Getenv("PORT")),
		JWTSecret:   get("JWT_SECRET"),
		UploadDir:   strings.TrimSpace(os.Getenv("UPLOAD_DIR")),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.UploadDir == "" {
		cfg.UploadDir = "./uploads"
	}
	origins := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if origins == "" {
		origins = "http://localhost:5173"
	}
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, o)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

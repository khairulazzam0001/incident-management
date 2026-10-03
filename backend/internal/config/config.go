// Package config loads process configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration for the API server.
type Config struct {
	DatabaseURL        string
	Port               string
	JWTSecret          string
	CORSAllowedOrigins []string
	UploadDir          string
	// SLATick is the SLA worker interval (SLA_TICK_SECONDS, default 60);
	// 0 disables the worker (SLA_WORKER_DISABLED=true).
	SLATick time.Duration
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
	cfg.SLATick = time.Minute
	if v := strings.TrimSpace(os.Getenv("SLA_TICK_SECONDS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 5 {
			return Config{}, fmt.Errorf("SLA_TICK_SECONDS harus angka >= 5")
		}
		cfg.SLATick = time.Duration(n) * time.Second
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("SLA_WORKER_DISABLED")), "true") {
		cfg.SLATick = 0
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

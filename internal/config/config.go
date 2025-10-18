package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr         string
	JWTSecret        string
	JWTIssuer        string
	QueueWorkers     int
	QueueBuf         int
	JobMaxDuration   time.Duration
	DatabaseURL      string
	StorageMode      string
	S3Bucket         string
	S3Endpoint       string
	S3Region         string
	AWSAccessKey     string
	AWSSecretKey     string
	S3ForcePathStyle bool
	LocalStorageDir  string
	LocalStorageURL  string
	OpenAIAPIKey     string
	RedisURL         string
	JWTTTLAccess     time.Duration
	JWTTTLRefresh    time.Duration
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			return i
		}
		slog.Warn("bad int env, using default", "key", key, "value", v)
	}
	return def
}

func getBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if v == "true" || v == "1" {
			return true
		}
		if v == "false" || v == "0" {
			return false
		}
		slog.Warn("bad bool env, using default", "key", key, "value", v)
	}
	return def
}

func mustDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
		slog.Warn("bad duration env, using default", "key", key, "value", v)
	}
	return def
}

func loadEnvFiles() {
	envFiles := []string{
		".env.local",
		".env",
	}

	// try to find .env files starting from current directory and going up
	currentDir, err := os.Getwd()
	if err != nil {
		slog.Debug("failed to get current directory", "error", err)
		return
	}

	// look in current directory and up to 3 parent directories
	searchDirs := []string{currentDir}
	for i := 0; i < 3; i++ {
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break // reached root
		}
		searchDirs = append(searchDirs, parent)
		currentDir = parent
	}

	loadedAny := false
	for _, dir := range searchDirs {
		for _, envFile := range envFiles {
			envPath := filepath.Join(dir, envFile)
			if _, err := os.Stat(envPath); err == nil {
				if err := godotenv.Load(envPath); err == nil {
					slog.Debug("loaded environment file", "path", envPath)
					loadedAny = true
				} else {
					slog.Debug("failed to load environment file", "path", envPath, "error", err)
				}
			}
		}
		if loadedAny {
			break // stop searching once we find .env files in a directory
		}
	}

	if !loadedAny {
		slog.Debug("no .env files found, using system environment variables only")
	}
}

func Load() Config {
	loadEnvFiles()
	return Config{
		HTTPAddr:         getenv("HTTP_ADDR", ":8080"),
		JWTSecret:        getenv("JWT_SECRET", "dev-secret-change-me"),
		JWTIssuer:        getenv("JWT_ISSUER", "smartheart"),
		QueueWorkers:     mustInt("QUEUE_WORKERS", 4),
		QueueBuf:         mustInt("QUEUE_BUFFER", 1024),
		JobMaxDuration:   mustDuration("JOB_MAX_DURATION", 30*time.Second),
		DatabaseURL:      getenv("DATABASE_URL", "postgres://user:password@localhost:5432/smartheart?sslmode=disable"),
		StorageMode:      getenv("STORAGE_MODE", "local"),
		S3Bucket:         getenv("S3_BUCKET", "smartheart-files"),
		S3Endpoint:       getenv("S3_ENDPOINT", "http://localhost:4566"),
		S3Region:         getenv("S3_REGION", "us-east-1"),
		AWSAccessKey:     getenv("AWS_ACCESS_KEY_ID", "test"),
		AWSSecretKey:     getenv("AWS_SECRET_ACCESS_KEY", "test"),
		S3ForcePathStyle: getBool("S3_FORCE_PATH_STYLE", true),
		LocalStorageDir:  getenv("LOCAL_STORAGE_DIR", "./uploads"),
		LocalStorageURL:  getenv("LOCAL_STORAGE_URL", "http://localhost:8080/files"),
		OpenAIAPIKey:     getenv("OPENAI_API_KEY", ""),
		RedisURL:         getenv("REDIS_URL", "redis://localhost:6379"),
		JWTTTLAccess:     mustDuration("JWT_TTL_ACCESS", 15*time.Minute),
		JWTTTLRefresh:    mustDuration("JWT_TTL_REFRESH", 7*24*time.Hour),
	}
}

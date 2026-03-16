package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// JWTConfig holds JWT-related settings.
type JWTConfig struct {
	Secret     string
	Issuer     string
	TTLAccess  time.Duration
	TTLRefresh time.Duration
}

// S3Config holds S3/object-storage settings.
type S3Config struct {
	Bucket         string
	Endpoint       string
	Region         string
	AWSAccessKey   string
	AWSSecretKey   string
	ForcePathStyle bool
}

// QueueConfig holds job queue settings.
type QueueConfig struct {
	Workers      int
	Buffer       int
	Mode         string // "memory" or "redis"
	Stream       string // Redis stream name
	Group        string // Redis consumer group name
	MaxDuration  time.Duration
	ClaimTimeout time.Duration // Time before stuck job is reclaimed
}

// DBConfig holds database connection settings.
type DBConfig struct {
	URL          string
	MaxConns     int
	MinConns     int
	QueryTimeout time.Duration
}

// StorageConfig holds file storage settings.
type StorageConfig struct {
	Mode     string
	LocalDir string
	LocalURL string
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	Origins     []string
	Credentials bool
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	RPM   int // max requests per minute per IP
	Burst int // burst size
}

// GPTConfig holds OpenAI/GPT settings.
type GPTConfig struct {
	APIKey string
	Model  string
}

type Config struct {
	HTTPAddr  string
	JWT       JWTConfig
	Queue     QueueConfig
	DB        DBConfig
	S3        S3Config
	Storage   StorageConfig
	GPT       GPTConfig
	RedisURL  string
	CORS      CORSConfig
	RateLimit RateLimitConfig
}

// Storage mode constants for compile-time safety.
const (
	StorageModeS3         = "s3"
	StorageModeAWS        = "aws"
	StorageModeLocalStack = "localstack"
	StorageModeLocal      = "local"
	StorageModeFilesystem = "filesystem"
)

// Queue mode constants.
const (
	QueueModeRedis  = "redis"
	QueueModeMemory = "memory"
)

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			return i
		}
		slog.Warn("bad int env, using default", "key", key, "value", v)
	}
	return def
}

func envBool(key string, def bool) bool {
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

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
		slog.Warn("bad duration env, using default", "key", key, "value", v)
	}
	return def
}

func envStringList(key string, def []string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		var result []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		if len(result) > 0 {
			return result
		}
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

	jwtSecret := envString("JWT_SECRET", "")
	if jwtSecret == "" {
		env := envString("APP_ENV", "development")
		if env == "production" || env == "prod" {
			slog.Error("JWT_SECRET must be set in production")
			os.Exit(1)
		}
		slog.Warn("JWT_SECRET is not set, using insecure default — DO NOT use in production")
		jwtSecret = "dev-secret-change-me-not-for-prod!"
	}

	dbURL := envString("DATABASE_URL", "")
	if dbURL == "" {
		slog.Warn("DATABASE_URL is not set, using insecure default — DO NOT use in production")
		dbURL = "postgres://user:password@localhost:5432/smartheart?sslmode=disable"
	}

	return Config{
		HTTPAddr: envString("HTTP_ADDR", ":8080"),
		JWT: JWTConfig{
			Secret:     jwtSecret,
			Issuer:     envString("JWT_ISSUER", "smartheart"),
			TTLAccess:  envDuration("JWT_TTL_ACCESS", 15*time.Minute),
			TTLRefresh: envDuration("JWT_TTL_REFRESH", 7*24*time.Hour),
		},
		Queue: QueueConfig{
			Workers:      envInt("QUEUE_WORKERS", 4),
			Buffer:       envInt("QUEUE_BUFFER", 1024),
			Mode:         envString("QUEUE_MODE", "redis"),
			Stream:       envString("QUEUE_STREAM", "smartheart:jobs"),
			Group:        envString("QUEUE_GROUP", "workers"),
			MaxDuration:  envDuration("JOB_MAX_DURATION", 30*time.Second),
			ClaimTimeout: envDuration("JOB_CLAIM_TIMEOUT", 60*time.Second),
		},
		DB: DBConfig{
			URL:          dbURL,
			MaxConns:     envInt("DB_MAX_CONNS", 20),
			MinConns:     envInt("DB_MIN_CONNS", 2),
			QueryTimeout: envDuration("DB_QUERY_TIMEOUT", 5*time.Second),
		},
		S3: S3Config{
			Bucket:         envString("S3_BUCKET", "smartheart-files"),
			Endpoint:       envString("S3_ENDPOINT", "http://localhost:4566"),
			Region:         envString("S3_REGION", "us-east-1"),
			AWSAccessKey:   envString("AWS_ACCESS_KEY_ID", ""),
			AWSSecretKey:   envString("AWS_SECRET_ACCESS_KEY", ""),
			ForcePathStyle: envBool("S3_FORCE_PATH_STYLE", true),
		},
		Storage: StorageConfig{
			Mode:     envString("STORAGE_MODE", "local"),
			LocalDir: envString("LOCAL_STORAGE_DIR", "./uploads"),
			LocalURL: envString("LOCAL_STORAGE_URL", "http://localhost:8081/files"),
		},
		GPT: GPTConfig{
			APIKey: envString("OPENAI_API_KEY", ""),
			Model:  envString("GPT_MODEL", "gpt-4o"),
		},
		RedisURL: envString("REDIS_URL", "redis://localhost:6379"),
		CORS: CORSConfig{
			Origins:     envStringList("CORS_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173"}),
			Credentials: envBool("CORS_CREDENTIALS", true),
		},
		RateLimit: RateLimitConfig{
			RPM:   envInt("RATE_LIMIT_RPM", 100),
			Burst: envInt("RATE_LIMIT_BURST", 20),
		},
	}
}

package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// load the values from .env file

type Config struct {

	// Application
	AppEnv 					string
	AppPort 					string

	// Database params
	DBHost					string
	DBPort					string
	DBName					string
	DBUser					string
	DBPassword				string
	DBSSLMode				string

	// Audio Processing
	FFmpegPath				string
	MaxUploadSizeMB		int
	SnippetMaxDurationS	int

	// Fingerprinting
	FFTSize					int
	SampleRate				int
	HopSize					int
	FingerprintFanOut		int
	CoherenceToleranceMS	int
	TopMatches				int
}

// func Load reads configuration from .env file and environment variables
func Load() (*Config, error) {
	// load .env if exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment variables")
	}

	cfg := &Config{
		// Application
		AppEnv:						getEnvString("APP_ENV", "development"),
		AppPort:						getEnvString("APP_PORT", "8080"),

		// Database parms
		DBHost:						getEnvString("DB_HOST", "localhost"),
		DBPort:						getEnvString("DB_PORT", "5432"),
		DBName:						getEnvString("DB_NAME", "djfingerprint"),
		DBSSLMode:					getEnvString("DB_SSLMODE", "disable"),
		DBUser:						getEnvString("DB_USER", "djapp"),
		DBPassword:					getEnvString("DB_PASSWORD", "moosed123"),

		// Audio
		FFmpegPath:					getEnvString("FFMPEG_PATH", "ffmpeg"),
		MaxUploadSizeMB:			getEnvInt("MAX_UPLOAD_SIZE_MB", 100),
		SnippetMaxDurationS:		getEnvInt("SNIPPET_MAX_DURATION_SEC", 30),

		// Fingerprinting
		FFTSize:						getEnvInt("FFT_SIZE", 1024),
		SampleRate:					getEnvInt("SAMPLE_RATE", 11025),
		HopSize:						getEnvInt("HOP_SIZE", 512),
		FingerprintFanOut:		getEnvInt("FINGERPRINT_FAN_OUT", 5),
		CoherenceToleranceMS:	getEnvInt("COHERENCE_TOLERANCE_MS", 100),
		TopMatches:					getEnvInt("TOP_MATCHES", 3),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}


// DSNPostgres returns the PostgreSQL connection string
func (c *Config) DSNPostgres() string {
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		c.DBHost,
		c.DBPort,
		c.DBName,
		c.DBUser,
		c.DBPassword,
		c.DBSSLMode,
	)
}

// IsDevelopment checks if the app runs in development mode
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

// validate checks that all required config values are present
func (c *Config) validate() error {
	if c.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD is required but not set!")
	}
	if c.DBName == "" {
		return fmt.Errorf("DB_NAME is required but not set!")
	}
	if c.FFTSize <= 0 {
		return fmt.Errorf("FFT_SIZE must be a positive integer")
	}
	if c.SampleRate <= 0 {
		return fmt.Errorf("SAMPLE_RATE must be a positive integer")
	}

	return nil
}

// return the env var value or the provided default
func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// return the env var as int or the provided default
func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("Warning: Invalid value for %s (%q), using default %d", key, val, defaultVal)
		return defaultVal
	}
	return parsed
}
package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                  string
	Host                  string
	BaseURL               string
	UploadDir             string
	MaxUploadSizeMB       int64
	APIKey                string
	RequireAuthForUpload  bool
	RequireAuthForDownload bool
}

func LoadConfig() *Config {
	// Muat file .env jika ada (opsional)
	_ = godotenv.Load()

	port := getEnv("PORT", "8085")
	host := getEnv("HOST", "0.0.0.0")
	baseURL := getEnv("BASE_URL", "")
	uploadDir := getEnv("UPLOAD_DIR", "./uploads")

	maxSizeStr := getEnv("MAX_UPLOAD_SIZE_MB", "100")
	maxSize, err := strconv.ParseInt(maxSizeStr, 10, 64)
	if err != nil || maxSize <= 0 {
		maxSize = 100
	}

	apiKey := getEnv("API_KEY", "android-storage-secret-key")
	authUpload := getEnvAsBool("REQUIRE_AUTH_FOR_UPLOAD", true)
	authDownload := getEnvAsBool("REQUIRE_AUTH_FOR_DOWNLOAD", false)

	return &Config{
		Port:                  port,
		Host:                  host,
		BaseURL:               baseURL,
		UploadDir:             uploadDir,
		MaxUploadSizeMB:       maxSize,
		APIKey:                apiKey,
		RequireAuthForUpload:  authUpload,
		RequireAuthForDownload: authDownload,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}

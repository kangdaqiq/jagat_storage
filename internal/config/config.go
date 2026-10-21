package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                   string
	Host                   string
	BaseURL                string
	UploadDir              string
	MaxUploadSizeMB        int64
	APIKey                 string
	AdminAPIKey            string
	DefaultUserQuotaMB     int64
	UsersFile              string
	RequireAuthForUpload   bool
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
	adminKey := getEnv("ADMIN_API_KEY", apiKey)

	quotaStr := getEnv("DEFAULT_USER_QUOTA_MB", "1000") // 1000 MB (1 GB)
	defaultQuota, err := strconv.ParseInt(quotaStr, 10, 64)
	if err != nil || defaultQuota < 0 {
		defaultQuota = 1000
	}

	usersFile := getEnv("USERS_FILE", "")

	authUpload := getEnvAsBool("REQUIRE_AUTH_FOR_UPLOAD", true)
	authDownload := getEnvAsBool("REQUIRE_AUTH_FOR_DOWNLOAD", false)

	return &Config{
		Port:                   port,
		Host:                   host,
		BaseURL:                baseURL,
		UploadDir:              uploadDir,
		MaxUploadSizeMB:        maxSize,
		APIKey:                 apiKey,
		AdminAPIKey:            adminKey,
		DefaultUserQuotaMB:     defaultQuota,
		UsersFile:              usersFile,
		RequireAuthForUpload:   authUpload,
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

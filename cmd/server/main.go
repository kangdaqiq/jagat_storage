package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"storage_server/internal/config"
	"storage_server/internal/handler"
	"storage_server/internal/middleware"
	"storage_server/internal/storage"
)

func main() {
	cfg := config.LoadConfig()

	// Inisialisasi file storage engine
	store, err := storage.NewFileStore(cfg.UploadDir)
	if err != nil {
		log.Fatalf("Gagal inisialisasi storage: %v", err)
	}

	// Inisialisasi user store berbasis JSON (multi-user & batas kuota)
	usersFilePath := cfg.UsersFile
	if usersFilePath == "" {
		usersFilePath = filepath.Join(cfg.UploadDir, "users.json")
	}

	userStore, err := storage.NewUserStore(usersFilePath, cfg.AdminAPIKey, cfg.DefaultUserQuotaMB)
	if err != nil {
		log.Fatalf("Gagal inisialisasi user store: %v", err)
	}

	// Sinkronisasi awal pemakaian kapasitas aktual seluruh user dari metadata berkas
	for _, u := range userStore.GetAllUsers() {
		actualUsed := store.GetTotalBytesByUser(u.ID)
		_ = userStore.SetUsage(u.ID, actualUsed)
	}

	r := gin.Default()

	// Pasang middleware global CORS
	r.Use(middleware.CORSMiddleware())

	// Limit request body
	r.MaxMultipartMemory = 32 << 20 // 32 MB buffer in memory

	// Handler instances
	healthH := handler.NewHealthHandler(cfg)
	fileH := handler.NewFileHandler(store, userStore, cfg)
	userH := handler.NewUserHandler(userStore, store, cfg)

	// Root endpoint informasi server
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service":        "Android Storage Server API",
			"status":         "online",
			"version":        "1.1.0",
			"multi_user":     true,
			"quota_per_user": true,
			"endpoints": gin.H{
				"health":          "GET /api/v1/health",
				"user_storage":    "GET /api/v1/user/storage (Lihat sisa & pemakaian kuota)",
				"upload_single":   "POST /api/v1/files/upload (Form: 'file', 'folder', 'name')",
				"upload_multiple": "POST /api/v1/files/upload-multiple (Form: 'files', 'folder')",
				"list_files":      "GET /api/v1/files?folder=&search=&page=1&limit=20",
				"file_info":       "GET /api/v1/files/:id",
				"preview_stream":  "GET /api/v1/files/:id/view",
				"thumbnail":       "GET /api/v1/files/:id/thumbnail",
				"download":        "GET /api/v1/files/:id/download",
				"delete":          "DELETE /api/v1/files/:id",
				"admin_users":     "GET/POST /api/v1/admin/users",
				"admin_quota":     "PUT /api/v1/admin/users/:id/quota",
			},
			"auth_info": gin.H{
				"header_key":             "X-API-Key atau Authorization: Bearer <key>",
				"required_for_upload":    cfg.RequireAuthForUpload,
				"required_for_download":  cfg.RequireAuthForDownload,
				"default_user_quota_mb":  cfg.DefaultUserQuotaMB,
			},
		})
	})

	api := r.Group("/api/v1")
	{
		api.GET("/health", healthH.HealthCheck)

		// Endpoint Info & Profil Penyimpanan User
		userGroup := api.Group("/user", middleware.UserAuth(userStore, true))
		{
			userGroup.GET("/storage", userH.GetMyStorage)
		}

		// Endpoint Khusus Administrator (Kelola Pengguna & Batas Kuota)
		adminGroup := api.Group("/admin", middleware.UserAuth(userStore, true), middleware.AdminOnly())
		{
			adminGroup.GET("/users", userH.ListUsers)
			adminGroup.POST("/users", userH.CreateUser)
			adminGroup.PUT("/users/:id/quota", userH.UpdateUserQuota)
			adminGroup.DELETE("/users/:id", userH.DeleteUser)
			adminGroup.POST("/users/recalculate", userH.RecalculateAllUsage)
		}

		// Endpoint Pengelolaan File
		files := api.Group("/files")
		{
			uploadAuth := middleware.UserAuth(userStore, cfg.RequireAuthForUpload)
			downloadAuth := middleware.UserAuth(userStore, cfg.RequireAuthForDownload)
			actionAuth := middleware.UserAuth(userStore, true)
			optionalAuth := middleware.UserAuth(userStore, false)

			// Upload
			files.POST("/upload", uploadAuth, fileH.UploadSingle)
			files.POST("/upload-multiple", uploadAuth, fileH.UploadMultiple)

			// Read & List (dengan isolasi data per user jika menggunakan API Key)
			files.GET("", optionalAuth, fileH.ListFiles)
			files.GET("/:id", optionalAuth, fileH.GetFileInfo)

			// Streaming & Download & Thumbnail
			files.GET("/:id/view", downloadAuth, fileH.ViewFile)
			files.GET("/:id/thumbnail", downloadAuth, fileH.ViewThumbnail)
			files.GET("/:id/download", downloadAuth, fileH.DownloadFile)

			// Delete
			files.DELETE("/:id", actionAuth, fileH.DeleteFile)
		}
	}

	address := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Printf("=====================================================")
	log.Printf(" Storage Server siap berjalan di http://%s", address)
	log.Printf(" Storage Directory      : %s", cfg.UploadDir)
	log.Printf(" Users Database (JSON)  : %s", usersFilePath)
	log.Printf(" Default User Quota     : %d MB", cfg.DefaultUserQuotaMB)
	log.Printf(" Max Upload Size / File : %d MB", cfg.MaxUploadSizeMB)
	log.Printf(" Admin API Key          : %s", cfg.AdminAPIKey)
	log.Printf("=====================================================")

	if err := r.Run(address); err != nil {
		log.Fatalf("Server gagal berjalan: %v", err)
	}
}
